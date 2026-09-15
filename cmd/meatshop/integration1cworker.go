package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/meatshop/internal/config"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/integration1cworker"
	"github.com/dronm/meatshop/internal/services"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newIntegration1CWorkerRuntime(
	cfg config.Config,
	pool *pgxpool.Pool,
) (*integration1cworker.Runtime, error) {
	workerCfg := cfg.Integration1C.Worker
	if !workerCfg.Enabled {
		return nil, nil
	}

	pollInterval, err := workerCfg.PollIntervalDuration()
	if err != nil {
		return nil, err
	}
	notifyReconnectInterval, err := workerCfg.NotifyReconnectIntervalDuration()
	if err != nil {
		return nil, err
	}
	requeueInterval, err := workerCfg.RequeueIntervalDuration()
	if err != nil {
		return nil, err
	}
	lockTimeout, err := workerCfg.LockTimeoutDuration()
	if err != nil {
		return nil, err
	}
	jobTimeout, err := workerCfg.JobTimeoutDuration()
	if err != nil {
		return nil, err
	}
	retryBaseDelay, err := workerCfg.RetryBaseDelayDuration()
	if err != nil {
		return nil, err
	}
	retryMaxDelay, err := workerCfg.RetryMaxDelayDuration()
	if err != nil {
		return nil, err
	}
	resultPollInterval, err := workerCfg.ResultPollIntervalDuration()
	if err != nil {
		return nil, err
	}

	runtime, err := integration1cworker.NewRuntime(
		pool,
		integration1cworker.RuntimeConfig{
			DSN:                     cfg.Database.Primary,
			APIURL:                  cfg.Integration1C.URL,
			WorkerID:                workerCfg.WorkerID,
			Concurrency:             workerCfg.Concurrency,
			PollInterval:            pollInterval,
			NotifyReconnectInterval: notifyReconnectInterval,
			RequeueInterval:         requeueInterval,
			LockTimeout:             lockTimeout,
			JobTimeout:              jobTimeout,
			RetryBaseDelay:          retryBaseDelay,
			RetryMaxDelay:           retryMaxDelay,
			ResultPollInterval:      resultPollInterval,
		},
		map[string]integration1cworker.ResultHandler{
			integration.CommandCreateOrder: services.HandleCreateOrder1CResult,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize integration 1c worker: %w", err)
	}

	runtime.SetJobGuard(func(ctx context.Context, job *integration1cworker.Job) (bool, error) {
		if job.Command != integration.CommandCreateOrder {
			return true, nil
		}
		orderID, orderVersion, err := services.OrderSyncJobVersion(job.Metadata)
		if err != nil || orderVersion <= 0 {
			// Legacy jobs do not carry an order version. Let them execute; newly
			// queued jobs are versioned and are protected by the stale-job guard.
			return true, nil
		}
		var currentVersion int64
		if err := pool.QueryRow(ctx, `SELECT version FROM public.orders WHERE id = $1`, orderID).Scan(&currentVersion); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return orderVersion == currentVersion, nil
	})

	return runtime, nil
}
