package maxbot

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"
)

const minSendInterval = 500 * time.Millisecond

type Sender struct {
	store    *Store
	client   *Client
	cfg      SenderConfig
	listener *Listener
	wake     chan struct{}
	lastSend time.Time
}

func NewSender(store *Store, client *Client, cfg SenderConfig) *Sender {
	return &Sender{
		store:    store,
		client:   client,
		cfg:      cfg,
		listener: NewListener(cfg.DSN, cfg.NotifyReconnectInterval),
		wake:     make(chan struct{}, 8),
	}
}

func (s *Sender) Run(ctx context.Context) error {
	if err := s.store.RequeueStale(ctx, s.cfg.LockTimeout); err != nil {
		return err
	}

	go s.listener.Run(ctx, s.wake)

	if err := s.drain(ctx); err != nil {
		slog.Error("initial MAX outgoing drain failed", "error", err)
	}

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case <-s.wake:
		}

		if err := s.store.RequeueStale(ctx, s.cfg.LockTimeout); err != nil {
			slog.Error("requeue stale MAX outgoing messages", "error", err)
			continue
		}
		if err := s.drain(ctx); err != nil {
			slog.Error("process MAX outgoing messages", "error", err)
		}
	}
}

func (s *Sender) drain(ctx context.Context) error {
	for ctx.Err() == nil {
		message, err := s.store.ClaimNext(ctx)
		if err != nil {
			return err
		}
		if message == nil {
			return nil
		}

		if err := s.waitRateLimit(ctx); err != nil {
			return err
		}
		sendErr := s.client.SendMessage(ctx, message.MaxUserID, message.Message)
		s.lastSend = time.Now()
		if sendErr == nil {
			if err := s.store.MarkSent(ctx, message.ID); err != nil {
				return err
			}
			continue
		}

		if message.AttemptCount >= s.cfg.MaxAttempts || !isRetryableSendError(sendErr) {
			if err := s.store.MarkFailed(ctx, message.ID, sendErr); err != nil {
				return err
			}
			slog.Error("MAX outgoing message permanently failed",
				"id", message.ID,
				"max_user_id", message.MaxUserID,
				"attempt", message.AttemptCount,
				"error", sendErr,
			)
			continue
		}

		delay := retryDelay(message.AttemptCount, s.cfg.RetryBaseDelay, s.cfg.RetryMaxDelay)
		if err := s.store.MarkRetry(ctx, message.ID, delay, sendErr); err != nil {
			return err
		}
		slog.Warn("MAX outgoing message scheduled for retry",
			"id", message.ID,
			"max_user_id", message.MaxUserID,
			"attempt", message.AttemptCount,
			"retry_in", delay,
			"error", sendErr,
		)
	}
	return ctx.Err()
}

func (s *Sender) waitRateLimit(ctx context.Context) error {
	if s.lastSend.IsZero() {
		return nil
	}
	wait := minSendInterval - time.Since(s.lastSend)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableSendError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return true
	}
	return apiErr.StatusCode == http.StatusRequestTimeout ||
		apiErr.StatusCode == http.StatusTooManyRequests ||
		apiErr.StatusCode >= 500
}

func retryDelay(attempt int, base, max time.Duration) time.Duration {
	if base <= 0 {
		base = 5 * time.Second
	}
	if max <= 0 {
		max = 5 * time.Minute
	}
	if attempt < 1 {
		attempt = 1
	}
	factor := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(base) * factor)
	if delay > max || delay < 0 {
		return max
	}
	return delay
}
