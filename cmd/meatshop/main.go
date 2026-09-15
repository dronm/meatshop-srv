package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dronm/ds/v4/pgds"
	"github.com/dronm/meatshop/internal/config"
	"github.com/dronm/meatshop/internal/httpapi"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/meatshop/internal/services"

	"github.com/dronm/webapp"
	"github.com/dronm/webapp/database"
	"github.com/dronm/webapp/events"
	weblogger "github.com/dronm/webapp/logger"
	websession "github.com/dronm/webapp/session"
	"github.com/dronm/webapp/ws"
)

var (
	ApplicationName        = "meatshop"
	ApplicationAuthorName  = "Andrey Mikhalevich"
	ApplicationAuthorEmail = "katrenplus@mail.ru"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		slog.Error(ApplicationName+" app failed to start", "error", err)
		os.Exit(1)
	}
}

func run() error {
	models.RegisterModelbindEnums()

	services.ProgAbout.Name = ApplicationName
	services.ProgAbout.Author.Name = ApplicationAuthorName
	services.ProgAbout.Author.Email = ApplicationAuthorEmail
	services.ProgAbout.Backend.Version = Version
	services.ProgAbout.Backend.Commit = Commit
	services.ProgAbout.Backend.BuildDate = BuildDate

	configPath := flag.String("config", config.DefaultConfigPath, "path to JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	if err := weblogger.Init(cfg.Debug.LogLevel, cfg.Debug.LogFormat); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}

	webapp.SetSQLDebug(cfg.Debug.SQLQueries)

	if cfg.Debug.LogConfig {
		slog.Info(
			"config loaded",
			"path", *configPath,
			"http_addr", cfg.HTTP.Addr,
			"sql_debug", cfg.Debug.SQLQueries,
		)
	}

	db, err := database.Open(cfg.Database, nil)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	permissionService := services.NewPermissionService(db)
	if err := permissionService.Load(context.Background()); err != nil {
		return fmt.Errorf("load permissions: %w", err)
	}

	permChecker := NewPermissionChecker(permissionService)

	var mainMenuCache services.MainMenuCache
	if cfg.MainMenu.CacheEnabled && cfg.Session.Enabled {
		cacheTTL, err := cfg.MainMenu.CacheTTLDuration()
		if err != nil {
			return err
		}

		mainMenuCache, err = services.NewMainMenuRedisCache(
			context.Background(),
			cfg.Session.Redis,
			cacheTTL,
		)
		if err != nil {
			slog.Warn("main menu redis cache is disabled", "error", err)
			mainMenuCache = nil
		} else {
			defer func() {
				if err := mainMenuCache.Close(); err != nil {
					slog.Warn("close main menu redis cache failed", "error", err)
				}
			}()
		}
	}

	oneCClient, err := registerServices(cfg, permChecker, mainMenuCache)
	if err != nil {
		return err
	}

	requestTimeout, err := cfg.RequestTimeout()
	if err != nil {
		return err
	}

	appCtx := &webapp.AppContext{
		DB:                db,
		PermissionChecker: permChecker,
		DefaultTimeout:    requestTimeout,
	}

	middlewares := []webapp.Middleware{
		webapp.RecoverMiddleware(),
		webapp.QueryIDMiddleware(""),
		webapp.RequestLoggerMiddleware(),
		webapp.CORSMiddleware(cfg.CORS.WebappConfig()),
	}

	var sessionResolver webapp.SessionResolver

	if cfg.Session.Enabled {
		sessionManager, err := websession.NewRedisManager(
			cfg.Session.WebappConfig(),
			cfg.Session.Redis.WebappConfig(),
		)
		if err != nil {
			return fmt.Errorf("initialize redis session manager: %w", err)
		}

		sessionResolver = sessionManager.Resolver
		middlewares = append(middlewares, webapp.SessionMiddleware(sessionResolver))

		slog.Info("session middleware attached")
	} else {
		slog.Warn("session is disabled; permission-protected routes will not be available")
	}

	prov, ok := db.(*pgds.Provider)
	if !ok {
		return fmt.Errorf("db provider should be pgds")
	}
	pool, err := prov.PrimaryPool()
	if err != nil {
		return fmt.Errorf("PrimaryPool():%v", err)
	}

	oneCWorker, err := newIntegration1CWorkerRuntime(cfg, pool)
	if err != nil {
		return err
	}
	var oneCWorkerCancel context.CancelFunc
	var oneCWorkerDone chan error
	if oneCWorker != nil {
		oneCWorkerCtx, cancel := context.WithCancel(context.Background())
		oneCWorkerCancel = cancel
		oneCWorkerDone = make(chan error, 1)
		go func() {
			oneCWorkerDone <- oneCWorker.Run(oneCWorkerCtx)
		}()
		slog.Info("embedded integration 1c worker started", "concurrency", cfg.Integration1C.Worker.Concurrency)
	}

	localEvents := map[string]events.Target{
		"ProductAgc.Process": {
			Service: "ProductAgc",
			Method:  "Process",
		},
	}

	eventServer := events.NewServer(events.Config{
		Pool:        pool,
		AppContext:  appCtx,
		LocalEvents: localEvents,
	})

	wsServer := ws.NewServer(ws.Config{
		AppContext:      appCtx,
		SessionResolver: sessionResolver,
		EventBus:        eventServer,
		CheckOrigin: func(origin string) bool {
			return true
		},
	})

	appCtx.EventPublisher = wsServer
	eventServer.SetPublisher(wsServer)

	webapp.MustRegisterService(
		"Event",
		&events.Service{},
		func(ctx webapp.ServiceContext) any {
			return events.NewService(ctx, wsServer)
		},
		webapp.WithCRUDNotifications(),
	)

	initCtx := context.Background()

	eventServer.Start(initCtx)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := eventServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("event server shutdown failed", "error", err)
		}
	}()

	router := webapp.MustBuildRouter(
		appCtx,
		httpapi.BuildRoutes(httpapi.Dependencies{
			DB:                  db,
			Integration1CClient: oneCClient,
		}),
		middlewares...,
	)

	readTimeout, err := cfg.HTTPReadTimeout()
	if err != nil {
		return err
	}

	writeTimeout, err := cfg.HTTPWriteTimeout()
	if err != nil {
		return err
	}

	idleTimeout, err := cfg.HTTPIdleTimeout()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/ws", wsServer.Handler())

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(ApplicationName+" app started", "addr", cfg.HTTP.Addr)
		serverErr <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		slog.Info("shutdown requested", "signal", sig.String())

	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}

	case err := <-oneCWorkerDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("integration 1c worker stopped: %w", err)
		}
		return fmt.Errorf("integration 1c worker stopped unexpectedly")
	}

	if oneCWorkerCancel != nil {
		oneCWorkerCancel()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	if oneCWorkerDone != nil {
		select {
		case err := <-oneCWorkerDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("shutdown integration 1c worker: %w", err)
			}
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown integration 1c worker: %w", shutdownCtx.Err())
		}
	}

	return nil
}
