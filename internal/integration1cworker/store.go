package integration1cworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ClaimNextJob(ctx context.Context, workerID string) (*Job, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const selectQuery = `
		SELECT
			id,
			command,
			params,
			correlation_id,
			metadata,
			priority,
			attempt_count,
			max_attempts,
			created_at,
			started_at
		FROM integration_1c.jobs
		WHERE status = 'queued'
			AND available_at <= clock_timestamp()
			AND attempt_count < max_attempts
		ORDER BY priority DESC, available_at, created_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`

	job := Job{}
	err = tx.QueryRow(ctx, selectQuery).Scan(
		&job.ID,
		&job.Command,
		&job.Params,
		&job.CorrelationID,
		&job.Metadata,
		&job.Priority,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.CreatedAt,
		&job.StartedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit empty claim transaction: %w", err)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("select next job: %w", err)
	}

	job.AttemptCount++
	job.WorkerID = workerID

	const updateQuery = `
		UPDATE integration_1c.jobs
		SET
			status = 'processing',
			attempt_count = $2,
			locked_at = clock_timestamp(),
			locked_by = $3,
			started_at = COALESCE(started_at, clock_timestamp())
		WHERE id = $1
		RETURNING started_at
	`
	if err := tx.QueryRow(ctx, updateQuery, job.ID, job.AttemptCount, workerID).Scan(&job.StartedAt); err != nil {
		return nil, fmt.Errorf("mark job processing: %w", err)
	}

	const attemptQuery = `
		INSERT INTO integration_1c.attempts (
			job_id,
			attempt_no,
			worker_id
		)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	if err := tx.QueryRow(ctx, attemptQuery, job.ID, job.AttemptCount, workerID).Scan(&job.AttemptID); err != nil {
		return nil, fmt.Errorf("insert attempt: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim transaction: %w", err)
	}
	return &job, nil
}

// FinishAttempt stores the raw technical outcome of the current attempt.
// When retryAt is non-nil, the job is returned to the queue. Otherwise a
// terminal row is inserted into integration_1c.results and the job is completed.
func (s *Store) FinishAttempt(
	ctx context.Context,
	job *Job,
	outcome AttemptOutcome,
	retryAt *time.Time,
	lastError string,
) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}
	if (outcome.HTTPStatus == nil) == (outcome.TransportError == "") {
		return fmt.Errorf("attempt outcome must contain either HTTP status or transport error")
	}

	headers, err := marshalHeaders(outcome.Headers)
	if err != nil {
		return err
	}

	var bodySize *int
	if outcome.HTTPStatus != nil {
		size := len(outcome.Body)
		bodySize = &size
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin finish transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const attemptQuery = `
		UPDATE integration_1c.attempts
		SET
			finished_at = clock_timestamp(),
			http_status = $3,
			content_type = NULLIF($4, ''),
			response_headers = $5,
			response_body = $6,
			response_body_size = $7,
			transport_error = NULLIF($8, '')
		WHERE id = $1
			AND job_id = $2
			AND finished_at IS NULL
	`
	tag, err := tx.Exec(
		ctx,
		attemptQuery,
		job.AttemptID,
		job.ID,
		outcome.HTTPStatus,
		outcome.ContentType,
		headers,
		outcome.Body,
		bodySize,
		outcome.TransportError,
	)
	if err != nil {
		return fmt.Errorf("finish attempt: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("attempt %d for job %d is missing or already finished", job.AttemptID, job.ID)
	}

	if retryAt != nil {
		const retryQuery = `
			UPDATE integration_1c.jobs
			SET
				status = 'queued',
				available_at = $2,
				locked_at = NULL,
				locked_by = NULL,
				last_error = NULLIF($3, '')
			WHERE id = $1
				AND status = 'processing'
				AND attempt_count = $4
		`
		tag, err := tx.Exec(ctx, retryQuery, job.ID, *retryAt, lastError, job.AttemptCount)
		if err != nil {
			return fmt.Errorf("requeue job: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("job %d is no longer owned by attempt %d", job.ID, job.AttemptCount)
		}
	} else {
		resultOutcome := "transport_error"
		if outcome.IsHTTPResponse() {
			resultOutcome = "http_response"
		}

		const resultQuery = `
			INSERT INTO integration_1c.results (
				job_id,
				attempt_id,
				outcome,
				http_status,
				content_type,
				response_headers,
				body,
				body_size,
				transport_error
			)
			VALUES (
				$1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, NULLIF($9, '')
			)
		`
		if _, err := tx.Exec(
			ctx,
			resultQuery,
			job.ID,
			job.AttemptID,
			resultOutcome,
			outcome.HTTPStatus,
			outcome.ContentType,
			headers,
			outcome.Body,
			bodySize,
			outcome.TransportError,
		); err != nil {
			return fmt.Errorf("insert terminal result: %w", err)
		}

		const completeQuery = `
			UPDATE integration_1c.jobs
			SET
				status = 'completed',
				completed_at = clock_timestamp(),
				locked_at = NULL,
				locked_by = NULL,
				last_error = NULLIF($2, '')
			WHERE id = $1
				AND status = 'processing'
				AND attempt_count = $3
		`
		tag, err := tx.Exec(ctx, completeQuery, job.ID, lastError, job.AttemptCount)
		if err != nil {
			return fmt.Errorf("complete job: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("job %d is no longer owned by attempt %d", job.ID, job.AttemptCount)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finish transaction: %w", err)
	}
	return nil
}

// RecoverStaleJobs requeues abandoned processing jobs. If the abandoned
// attempt already exhausted max_attempts, a terminal transport_error result is
// created instead. This makes crashes recoverable without Redis or a broker.
func (s *Store) RecoverStaleJobs(ctx context.Context, lockTimeout time.Duration, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin stale recovery transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	type staleJob struct {
		ID           int64
		AttemptCount int
		MaxAttempts  int
		AttemptID    int64
	}

	const selectQuery = `
		SELECT
			j.id,
			j.attempt_count,
			j.max_attempts,
			a.id
		FROM integration_1c.jobs AS j
		JOIN integration_1c.attempts AS a
			ON a.job_id = j.id
			AND a.attempt_no = j.attempt_count
		WHERE j.status = 'processing'
			AND j.locked_at < clock_timestamp() - ($1::double precision * interval '1 second')
		ORDER BY j.locked_at, j.id
		FOR UPDATE OF j SKIP LOCKED
		LIMIT $2
	`

	rows, err := tx.Query(ctx, selectQuery, lockTimeout.Seconds(), limit)
	if err != nil {
		return 0, fmt.Errorf("select stale jobs: %w", err)
	}

	staleJobs := make([]staleJob, 0)
	for rows.Next() {
		var item staleJob
		if err := rows.Scan(&item.ID, &item.AttemptCount, &item.MaxAttempts, &item.AttemptID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan stale job: %w", err)
		}
		staleJobs = append(staleJobs, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate stale jobs: %w", err)
	}
	rows.Close()

	const abandonedError = "worker lock expired before attempt completion"
	for _, item := range staleJobs {
		if _, err := tx.Exec(ctx, `
			UPDATE integration_1c.attempts
			SET
				finished_at = clock_timestamp(),
				transport_error = $2
			WHERE id = $1
				AND finished_at IS NULL
		`, item.AttemptID, abandonedError); err != nil {
			return 0, fmt.Errorf("finish abandoned attempt %d: %w", item.AttemptID, err)
		}

		if item.AttemptCount < item.MaxAttempts {
			if _, err := tx.Exec(ctx, `
				UPDATE integration_1c.jobs
				SET
					status = 'queued',
					available_at = clock_timestamp(),
					locked_at = NULL,
					locked_by = NULL,
					last_error = $2
				WHERE id = $1
			`, item.ID, abandonedError); err != nil {
				return 0, fmt.Errorf("requeue stale job %d: %w", item.ID, err)
			}
			continue
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO integration_1c.results (
				job_id,
				attempt_id,
				outcome,
				transport_error
			)
			VALUES ($1, $2, 'transport_error', $3)
		`, item.ID, item.AttemptID, abandonedError); err != nil {
			return 0, fmt.Errorf("insert stale terminal result for job %d: %w", item.ID, err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE integration_1c.jobs
			SET
				status = 'completed',
				completed_at = clock_timestamp(),
				locked_at = NULL,
				locked_by = NULL,
				last_error = $2
			WHERE id = $1
		`, item.ID, abandonedError); err != nil {
			return 0, fmt.Errorf("complete stale job %d: %w", item.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit stale recovery transaction: %w", err)
	}
	return len(staleJobs), nil
}

func marshalHeaders(headers map[string][]string) ([]byte, error) {
	if headers == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("marshal response headers: %w", err)
	}
	return encoded, nil
}
