package integration1cworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type JobStore interface {
	ClaimNextJob(ctx context.Context, workerID string) (*Job, error)
	FinishAttempt(ctx context.Context, job *Job, outcome AttemptOutcome, retryAt *time.Time, lastError string) error
	RecoverStaleJobs(ctx context.Context, lockTimeout time.Duration, limit int) (int, error)
}

type Executor interface {
	Execute(ctx context.Context, command string, params json.RawMessage) (Response, error)
}

type JobGuard func(ctx context.Context, job *Job) (bool, error)

type WorkerService struct {
	store           JobStore
	executor        Executor
	workerID        string
	concurrency     int
	pollInterval    time.Duration
	requeueInterval time.Duration
	lockTimeout     time.Duration
	jobTimeout      time.Duration
	retryBaseDelay  time.Duration
	retryMaxDelay   time.Duration
	wake            <-chan struct{}
	guard           JobGuard
}

func NewWorkerService(
	jobStore JobStore,
	executor Executor,
	workerID string,
	concurrency int,
	pollInterval time.Duration,
	requeueInterval time.Duration,
	lockTimeout time.Duration,
	jobTimeout time.Duration,
	retryBaseDelay time.Duration,
	retryMaxDelay time.Duration,
	wake <-chan struct{},
) *WorkerService {
	return &WorkerService{
		store:           jobStore,
		executor:        executor,
		workerID:        workerID,
		concurrency:     concurrency,
		pollInterval:    pollInterval,
		requeueInterval: requeueInterval,
		lockTimeout:     lockTimeout,
		jobTimeout:      jobTimeout,
		retryBaseDelay:  retryBaseDelay,
		retryMaxDelay:   retryMaxDelay,
		wake:            wake,
	}
}

func (s *WorkerService) SetJobGuard(guard JobGuard) {
	s.guard = guard
}

func (s *WorkerService) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.requeueLoop(ctx)
	}()

	for i := 0; i < s.concurrency; i++ {
		workerID := fmt.Sprintf("%s#%d", s.workerID, i+1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.workerLoop(ctx, workerID)
		}()
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (s *WorkerService) workerLoop(ctx context.Context, workerID string) {
	for ctx.Err() == nil {
		job, err := s.store.ClaimNextJob(ctx, workerID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("claim next job", "workerID", workerID, "err", err)
			if !s.waitForWork(ctx) {
				return
			}
			continue
		}

		if job == nil {
			if !s.waitForWork(ctx) {
				return
			}
			continue
		}

		if err := s.handleJob(ctx, job); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error(
				"handle job",
				"jobID", job.ID,
				"command", job.Command,
				"attempt", job.AttemptCount,
				"err", err,
			)
		}
	}
}

func (s *WorkerService) handleJob(ctx context.Context, job *Job) error {
	jobCtx, cancel := context.WithTimeout(ctx, s.jobTimeout)
	defer cancel()

	if s.guard != nil {
		execute, err := s.guard(jobCtx, job)
		if err != nil {
			return fmt.Errorf("guard job: %w", err)
		}
		if !execute {
			status := http.StatusNoContent
			if err := s.store.FinishAttempt(ctx, job, AttemptOutcome{HTTPStatus: &status}, nil, ""); err != nil {
				return fmt.Errorf("finish skipped attempt: %w", err)
			}
			slog.Info("obsolete job skipped", "jobID", job.ID, "command", job.Command)
			return nil
		}
	}

	response, executeErr := s.executor.Execute(jobCtx, job.Command, job.Params)

	// If the whole service is stopping, leave the processing row untouched.
	// Stale-job recovery will safely reclaim it on the next process start.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	outcome := AttemptOutcome{}
	lastError := ""
	retryable := false

	if executeErr != nil {
		outcome.TransportError = executeErr.Error()
		lastError = executeErr.Error()
		retryable = true
	} else {
		status := response.StatusCode
		outcome.HTTPStatus = &status
		outcome.ContentType = response.Header.Get("Content-Type")
		outcome.Headers = map[string][]string(response.Header)
		outcome.Body = response.Body
		retryable = shouldRetryHTTPStatus(status)
		if status < 200 || status >= 300 {
			lastError = fmt.Sprintf("HTTP %d", status)
		}
	}

	var retryAt *time.Time
	if retryable && job.AttemptCount < job.MaxAttempts {
		next := time.Now().Add(backoff(job.AttemptCount, s.retryBaseDelay, s.retryMaxDelay))
		retryAt = &next
	}

	if err := s.store.FinishAttempt(ctx, job, outcome, retryAt, lastError); err != nil {
		return fmt.Errorf("finish attempt: %w", err)
	}

	if retryAt != nil {
		slog.Warn(
			"job scheduled for retry",
			"jobID", job.ID,
			"command", job.Command,
			"attempt", job.AttemptCount,
			"maxAttempts", job.MaxAttempts,
			"availableAt", *retryAt,
			"reason", lastError,
		)
		return nil
	}

	if outcome.HTTPStatus != nil {
		slog.Info(
			"job completed with HTTP result",
			"jobID", job.ID,
			"command", job.Command,
			"attempt", job.AttemptCount,
			"httpStatus", *outcome.HTTPStatus,
			"responseBytes", len(outcome.Body),
		)
	} else {
		slog.Error(
			"job completed with terminal transport error",
			"jobID", job.ID,
			"command", job.Command,
			"attempt", job.AttemptCount,
			"err", outcome.TransportError,
		)
	}

	return nil
}

func (s *WorkerService) waitForWork(ctx context.Context) bool {
	timer := time.NewTimer(s.pollInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	case <-s.wake:
		return true
	}
}

func (s *WorkerService) requeueLoop(ctx context.Context) {
	s.recoverStale(ctx)

	ticker := time.NewTicker(s.requeueInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.recoverStale(ctx)
		}
	}
}

func (s *WorkerService) recoverStale(ctx context.Context) {
	for ctx.Err() == nil {
		count, err := s.store.RecoverStaleJobs(ctx, s.lockTimeout, 100)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("recover stale jobs", "err", err)
			}
			return
		}
		if count == 0 {
			return
		}
		slog.Warn("recovered stale jobs", "count", count)
		if count < 100 {
			return
		}
	}
}

func shouldRetryHTTPStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusTooEarly ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func backoff(attempt int, base time.Duration, max time.Duration) time.Duration {
	if attempt <= 1 {
		return base
	}

	delay := base
	for i := 1; i < attempt; i++ {
		if delay >= max/2 {
			return max
		}
		delay *= 2
	}
	if delay > max {
		return max
	}
	return delay
}
