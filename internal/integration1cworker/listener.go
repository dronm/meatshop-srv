package integration1cworker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	JobsChannel    = "integration_1c_jobs"
	ResultsChannel = "integration_1c_results"
)

type Listener struct {
	dsn               string
	reconnectInterval time.Duration
}

func NewListener(dsn string, reconnectInterval time.Duration) *Listener {
	return &Listener{
		dsn:               dsn,
		reconnectInterval: reconnectInterval,
	}
}

func (l *Listener) RunJobs(ctx context.Context, wake chan<- struct{}) {
	l.run(ctx, JobsChannel, wake)
}

func (l *Listener) RunResults(ctx context.Context, wake chan<- struct{}) {
	l.run(ctx, ResultsChannel, wake)
}

func (l *Listener) run(ctx context.Context, channel string, wake chan<- struct{}) {
	for ctx.Err() == nil {
		if err := l.runConnection(ctx, channel, wake); err != nil && ctx.Err() == nil {
			slog.Error("1c integration notification listener disconnected", "channel", channel, "err", err)
		}

		if !sleepContext(ctx, l.reconnectInterval) {
			return
		}
	}
}

func (l *Listener) runConnection(ctx context.Context, channel string, wake chan<- struct{}) error {
	conn, err := pgx.Connect(ctx, l.dsn)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close(context.Background())
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}

	slog.Info("listening for 1c integration notifications", "channel", channel)
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return err
		}
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
