package integration1cworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type Result struct {
	ID             int64
	JobID          int64
	AttemptID      int64
	Outcome        string
	HTTPStatus     *int
	ContentType    string
	Body           []byte
	TransportError string
	CreatedAt      time.Time
	Command        string
	Params         json.RawMessage
	CorrelationID  *string
	Metadata       json.RawMessage
}

type ResultHandler func(ctx context.Context, tx pgx.Tx, result Result) error

type ResultConsumer struct {
	store        *Store
	consumerID   string
	handlers     map[string]ResultHandler
	commands     []string
	pollInterval time.Duration
	wake         <-chan struct{}
}

func NewResultConsumer(
	store *Store,
	consumerID string,
	handlers map[string]ResultHandler,
	pollInterval time.Duration,
	wake <-chan struct{},
) *ResultConsumer {
	commands := make([]string, 0, len(handlers))
	for command := range handlers {
		commands = append(commands, command)
	}
	sort.Strings(commands)

	return &ResultConsumer{
		store:        store,
		consumerID:   consumerID,
		handlers:     handlers,
		commands:     commands,
		pollInterval: pollInterval,
		wake:         wake,
	}
}

func (c *ResultConsumer) Run(ctx context.Context) error {
	if len(c.commands) == 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	for ctx.Err() == nil {
		processed, err := c.consumeOne(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("consume 1c integration result", "err", err)
			if !c.wait(ctx) {
				return ctx.Err()
			}
			continue
		}
		if processed {
			continue
		}
		if !c.wait(ctx) {
			return ctx.Err()
		}
	}

	return ctx.Err()
}

func (c *ResultConsumer) consumeOne(ctx context.Context) (bool, error) {
	tx, err := c.store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin result transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const query = `
		SELECT
			r.id,
			r.job_id,
			r.attempt_id,
			r.outcome,
			r.http_status,
			COALESCE(r.content_type, ''),
			COALESCE(r.body, ''::bytea),
			COALESCE(r.transport_error, ''),
			r.created_at,
			j.command,
			j.params,
			j.correlation_id,
			j.metadata
		FROM integration_1c.results AS r
		JOIN integration_1c.jobs AS j ON j.id = r.job_id
		WHERE r.consumed_at IS NULL
			AND j.command = ANY($1::text[])
		ORDER BY r.created_at, r.id
		FOR UPDATE OF r SKIP LOCKED
		LIMIT 1
	`

	var result Result
	err = tx.QueryRow(ctx, query, c.commands).Scan(
		&result.ID,
		&result.JobID,
		&result.AttemptID,
		&result.Outcome,
		&result.HTTPStatus,
		&result.ContentType,
		&result.Body,
		&result.TransportError,
		&result.CreatedAt,
		&result.Command,
		&result.Params,
		&result.CorrelationID,
		&result.Metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			if err := tx.Commit(ctx); err != nil {
				return false, fmt.Errorf("commit empty result transaction: %w", err)
			}
			return false, nil
		}
		return false, fmt.Errorf("select integration result: %w", err)
	}

	handler := c.handlers[result.Command]
	if handler == nil {
		return false, fmt.Errorf("no result handler registered for command %q", result.Command)
	}
	if err := handler(ctx, tx, result); err != nil {
		return false, fmt.Errorf("handle result %d for command %q: %w", result.ID, result.Command, err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE integration_1c.results
		SET
			consumed_at = clock_timestamp(),
			consumed_by = $2
		WHERE id = $1
			AND consumed_at IS NULL
	`, result.ID, c.consumerID)
	if err != nil {
		return false, fmt.Errorf("mark integration result consumed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return false, fmt.Errorf("integration result %d is already consumed", result.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit result transaction: %w", err)
	}
	return true, nil
}

func (c *ResultConsumer) wait(ctx context.Context) bool {
	timer := time.NewTimer(c.pollInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	case <-c.wake:
		return true
	}
}
