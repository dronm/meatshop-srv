package integration1cworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RuntimeConfig struct {
	DSN                     string
	APIURL                  string
	WorkerID                string
	Concurrency             int
	PollInterval            time.Duration
	NotifyReconnectInterval time.Duration
	RequeueInterval         time.Duration
	LockTimeout             time.Duration
	JobTimeout              time.Duration
	RetryBaseDelay          time.Duration
	RetryMaxDelay           time.Duration
	ResultPollInterval      time.Duration
}

type Runtime struct {
	worker         *WorkerService
	resultConsumer *ResultConsumer
	listener       *Listener
	jobWake        chan struct{}
	resultWake     chan struct{}
}

func NewRuntime(pool *pgxpool.Pool, cfg RuntimeConfig, handlers map[string]ResultHandler) (*Runtime, error) {
	if pool == nil {
		return nil, fmt.Errorf("integration 1c worker pool is required")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("integration 1c worker DSN is required")
	}
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("integration 1c worker API URL is required")
	}
	if cfg.Concurrency <= 0 {
		return nil, fmt.Errorf("integration 1c worker concurrency should be positive")
	}
	if cfg.PollInterval <= 0 || cfg.NotifyReconnectInterval <= 0 || cfg.RequeueInterval <= 0 ||
		cfg.LockTimeout <= 0 || cfg.JobTimeout <= 0 || cfg.RetryBaseDelay <= 0 ||
		cfg.RetryMaxDelay <= 0 || cfg.ResultPollInterval <= 0 {
		return nil, fmt.Errorf("integration 1c worker durations should be positive")
	}
	if cfg.LockTimeout <= cfg.JobTimeout {
		return nil, fmt.Errorf("integration 1c worker lock timeout should be greater than job timeout")
	}
	if cfg.RetryMaxDelay < cfg.RetryBaseDelay {
		return nil, fmt.Errorf("integration 1c worker retry max delay should be >= retry base delay")
	}

	workerID := cfg.WorkerID
	if workerID == "" {
		host, _ := os.Hostname()
		workerID = fmt.Sprintf("meatshop@%s:%d", host, os.Getpid())
	}

	gatewayClient, err := NewGatewayClient(cfg.APIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create integration 1c gateway client: %w", err)
	}

	store := NewStore(pool)
	jobWake := make(chan struct{}, cfg.Concurrency*4)
	resultWake := make(chan struct{}, 8)

	return &Runtime{
		worker: NewWorkerService(
			store,
			gatewayClient,
			workerID,
			cfg.Concurrency,
			cfg.PollInterval,
			cfg.RequeueInterval,
			cfg.LockTimeout,
			cfg.JobTimeout,
			cfg.RetryBaseDelay,
			cfg.RetryMaxDelay,
			jobWake,
		),
		resultConsumer: NewResultConsumer(
			store,
			workerID,
			handlers,
			cfg.ResultPollInterval,
			resultWake,
		),
		listener:   NewListener(cfg.DSN, cfg.NotifyReconnectInterval),
		jobWake:    jobWake,
		resultWake: resultWake,
	}, nil
}

func (r *Runtime) SetJobGuard(guard JobGuard) {
	r.worker.SetJobGuard(guard)
}

func (r *Runtime) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	workerErr := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		r.listener.RunJobs(runCtx, r.jobWake)
	}()
	go func() {
		defer wg.Done()
		r.listener.RunResults(runCtx, r.resultWake)
	}()

	wg.Add(2)
	go func() {
		defer wg.Done()
		workerErr <- r.worker.Run(runCtx)
	}()
	go func() {
		defer wg.Done()
		workerErr <- r.resultConsumer.Run(runCtx)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
	case err := <-workerErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			runErr = err
		}
	}

	cancel()
	wg.Wait()
	if errors.Is(runErr, context.Canceled) {
		return ctx.Err()
	}
	return runErr
}
