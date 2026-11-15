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
	"github.com/dronm/meatshop/internal/appconstants"
	"github.com/dronm/meatshop/internal/config"
	"github.com/dronm/meatshop/internal/maxbot"
	"github.com/dronm/webapp/database"
	weblogger "github.com/dronm/webapp/logger"
)

func main() {
	if err := run(); err != nil {
		slog.Error("MAX bot failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", config.DefaultConfigPath, "path to JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if err := weblogger.Init(cfg.Debug.LogLevel, cfg.Debug.LogFormat); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	if cfg.MAX.BotToken == "" {
		return fmt.Errorf("max.bot_token is required")
	}

	db, err := database.Open(cfg.Database, nil)
	if err != nil {
		return fmt.Errorf("open MAX database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close MAX database", "error", err)
		}
	}()
	provider, ok := db.(*pgds.Provider)
	if !ok {
		return fmt.Errorf("MAX database provider must be pgds")
	}
	pool, err := provider.PrimaryPool()
	if err != nil {
		return fmt.Errorf("MAX primary database pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("ping MAX database: %w", err)
	}

	projectConstants, err := appconstants.Open(context.Background(), db, cfg.Database.Primary)
	if err != nil {
		return fmt.Errorf("initialize MAX constants (apply database migrations first): %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := projectConstants.Close(ctx); err != nil {
			slog.Error("close MAX constants", "error", err)
		}
	}()

	client, err := maxbot.NewClient(cfg.MAX.BotToken)
	if err != nil {
		return err
	}
	botInfoCtx, botInfoCancel := context.WithTimeout(context.Background(), 15*time.Second)
	botInfo, err := client.GetMe(botInfoCtx)
	botInfoCancel()
	if err != nil {
		return err
	}
	slog.Info("MAX bot identified", "bot_id", botInfo.UserID)

	if cfg.MAX.WebhookURL != "" {
		webhookCtx, webhookCancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := client.ConfigureWebhook(webhookCtx, cfg.MAX.WebhookURL, cfg.MAX.WebhookSecret); err != nil {
			webhookCancel()
			return err
		}
		webhookCancel()
		slog.Info("MAX webhook subscription configured", "url", cfg.MAX.WebhookURL)
	}
	store := maxbot.NewStore(pool)
	handler, err := maxbot.NewHandler(store, cfg.MAX.WebhookSecret, botInfo.UserID)
	if err != nil {
		return err
	}

	pollInterval, err := cfg.MAX.SenderPollIntervalDuration()
	if err != nil {
		return err
	}
	notifyReconnectInterval, err := cfg.MAX.SenderNotifyReconnectIntervalDuration()
	if err != nil {
		return err
	}
	lockTimeout, err := cfg.MAX.SenderLockTimeoutDuration()
	if err != nil {
		return err
	}
	retryBaseDelay, err := cfg.MAX.SenderRetryBaseDelayDuration()
	if err != nil {
		return err
	}
	retryMaxDelay, err := cfg.MAX.SenderRetryMaxDelayDuration()
	if err != nil {
		return err
	}

	sender := maxbot.NewSender(store, client, maxbot.SenderConfig{
		DSN:                     cfg.Database.Primary,
		PollInterval:            pollInterval,
		NotifyReconnectInterval: notifyReconnectInterval,
		LockTimeout:             lockTimeout,
		RetryBaseDelay:          retryBaseDelay,
		RetryMaxDelay:           retryMaxDelay,
		MaxAttempts:             cfg.MAX.SenderMaxAttempts,
	})

	mux := http.NewServeMux()
	mux.Handle("/webhook", handler)
	server := &http.Server{
		Addr:         cfg.MAX.HTTPAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("MAX bot webhook server started", "addr", cfg.MAX.HTTPAddr)
		serverErr <- server.ListenAndServe()
	}()

	senderErr := make(chan error, 1)
	go func() {
		slog.Info("MAX outgoing sender started")
		senderErr <- sender.Run(runCtx)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		slog.Info("MAX bot shutdown requested", "signal", sig.String())
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-senderErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("MAX sender stopped: %w", err)
		}
		return fmt.Errorf("MAX sender stopped unexpectedly")
	}

	cancel()
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return server.Shutdown(ctx)
}
