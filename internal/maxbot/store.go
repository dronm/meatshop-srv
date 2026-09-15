package maxbot

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

func (s *Store) SaveUpdate(ctx context.Context, raw json.RawMessage, update Update, welcome json.RawMessage) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("MAX store is not initialized")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	maxUserID := updateMaxUserID(update)
	maxChatID := updateMaxChatID(update)
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.max_in_messages (
			update_type,
			max_user_id,
			max_chat_id,
			message
		)
		VALUES ($1, $2, $3, $4::jsonb)
	`, update.UpdateType, maxUserID, maxChatID, string(raw)); err != nil {
		return fmt.Errorf("persist MAX incoming update: %w", err)
	}

	if update.UpdateType == "bot_started" {
		if update.User == nil || update.User.UserID <= 0 {
			return fmt.Errorf("MAX bot_started update does not contain a valid user")
		}
		rawUser, err := extractRawUser(raw)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.max_users (
				max_user_id,
				username,
				app_username,
				raw_user,
				is_active
			)
			VALUES ($1, $2, $3, $4::jsonb, true)
			ON CONFLICT (max_user_id) DO UPDATE
			SET
				username = EXCLUDED.username,
				raw_user = EXCLUDED.raw_user,
				is_active = true
		`,
			update.User.UserID,
			normalized(update.User.Username),
			initialAppUsername(update.User.Username),
			string(rawUser),
		); err != nil {
			return fmt.Errorf("upsert MAX user from bot_started: %w", err)
		}

		if len(welcome) == 0 {
			return fmt.Errorf("MAX welcome message is not configured")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.max_out_messages (
				max_user_id,
				message,
				metadata
			)
			VALUES (
				$1,
				$2::jsonb,
				jsonb_build_object('source', 'bot_started', 'update_type', $3::text)
			)
		`, update.User.UserID, string(welcome), update.UpdateType); err != nil {
			return fmt.Errorf("queue MAX welcome message: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit MAX incoming update: %w", err)
	}
	return nil
}

func (s *Store) RequeueStale(ctx context.Context, lockTimeout time.Duration) error {
	if lockTimeout <= 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'pending',
			locked_at = NULL,
			next_attempt_at = now(),
			error_str = COALESCE(error_str, 'sender lock expired')
		WHERE status = 'sending'
			AND locked_at IS NOT NULL
			AND locked_at < now() - $1::interval
	`, durationInterval(lockTimeout))
	if err != nil {
		return fmt.Errorf("requeue stale MAX messages: %w", err)
	}
	return nil
}

func (s *Store) ClaimNext(ctx context.Context) (*OutMessage, error) {
	var result OutMessage
	err := s.pool.QueryRow(ctx, `
		WITH next_message AS (
			SELECT id
			FROM public.max_out_messages
			WHERE status = 'pending'
				AND (next_attempt_at IS NULL OR next_attempt_at <= now())
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE public.max_out_messages AS m
		SET
			status = 'sending',
			locked_at = now(),
			attempt_count = m.attempt_count + 1,
			error_str = NULL
		FROM next_message
		WHERE m.id = next_message.id
		RETURNING m.id, m.max_user_id, m.message, m.attempt_count
	`).Scan(&result.ID, &result.MaxUserID, &result.Message, &result.AttemptCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claim MAX outgoing message: %w", err)
	}
	return &result, nil
}

func (s *Store) MarkSent(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'sent',
			sent_at = now(),
			locked_at = NULL,
			next_attempt_at = NULL,
			error_str = NULL
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark MAX message sent: %w", err)
	}
	return nil
}

func (s *Store) MarkRetry(ctx context.Context, id int64, delay time.Duration, sendErr error) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'pending',
			locked_at = NULL,
			next_attempt_at = now() + $2::interval,
			error_str = $3
		WHERE id = $1
	`, id, durationInterval(delay), truncateError(sendErr))
	if err != nil {
		return fmt.Errorf("schedule MAX message retry: %w", err)
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id int64, sendErr error) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE public.max_out_messages
		SET
			status = 'failed',
			locked_at = NULL,
			next_attempt_at = NULL,
			error_str = $2
		WHERE id = $1
	`, id, truncateError(sendErr))
	if err != nil {
		return fmt.Errorf("mark MAX message failed: %w", err)
	}
	return nil
}

func initialAppUsername(username *string) string {
	normalizedUsername := normalized(username)
	if normalizedUsername == nil {
		return "Не задано"
	}
	return *normalizedUsername
}

func extractRawUser(raw json.RawMessage) (json.RawMessage, error) {
	var envelope struct {
		User json.RawMessage `json:"user"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode MAX update user: %w", err)
	}
	if len(envelope.User) == 0 || string(envelope.User) == "null" {
		return nil, fmt.Errorf("MAX update does not contain user JSON")
	}
	return envelope.User, nil
}

func updateMaxUserID(update Update) any {
	if update.User != nil && update.User.UserID > 0 {
		return update.User.UserID
	}
	if update.Message != nil && update.Message.Sender != nil && update.Message.Sender.UserID > 0 {
		return update.Message.Sender.UserID
	}
	return nil
}

func updateMaxChatID(update Update) any {
	if update.ChatID != nil && *update.ChatID != 0 {
		return *update.ChatID
	}
	if update.Message != nil && update.Message.Recipient != nil && update.Message.Recipient.ChatID != nil && *update.Message.Recipient.ChatID != 0 {
		return *update.Message.Recipient.ChatID
	}
	return nil
}

func durationInterval(value time.Duration) string {
	return fmt.Sprintf("%f seconds", value.Seconds())
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	const maxLength = 4000
	if len(value) > maxLength {
		return value[:maxLength]
	}
	return value
}
