package maxbot

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

const outMessagesChannel = "max_out_messages"

type Listener struct {
	dsn               string
	reconnectInterval time.Duration
}

func NewListener(dsn string, reconnectInterval time.Duration) *Listener {
	return &Listener{dsn: dsn, reconnectInterval: reconnectInterval}
}

func (l *Listener) Run(ctx context.Context, wake chan<- struct{}) {
	for ctx.Err() == nil {
		if err := l.runConnection(ctx, wake); err != nil && ctx.Err() == nil {
			slog.Error("MAX outgoing notification listener disconnected", "error", err)
		}
		if !sleepContext(ctx, l.reconnectInterval) {
			return
		}
	}
}

func (l *Listener) runConnection(ctx context.Context, wake chan<- struct{}) error {
	conn, err := pgx.Connect(ctx, l.dsn)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close(context.Background())
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+outMessagesChannel); err != nil {
		return err
	}
	slog.Info("listening for MAX outgoing messages", "channel", outMessagesChannel)

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
	if duration <= 0 {
		duration = time.Second
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
