package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dronm/ds/v4"
	"github.com/dronm/webapp"
)

const productionMaxChatConstantCode = "production_max_chat_id"

// enqueueProductionMessages must use the production document's transaction.
// Destination and complete message text are fixed in the outbox when saved.
func enqueueProductionMessages(
	ctx context.Context,
	tx ds.Querier,
	transferID int,
	version int64,
	messages []string,
) error {
	if transferID <= 0 || version <= 0 || len(messages) == 0 {
		return webapp.BadRequest("Некорректные параметры отправки заявки в производство.", nil)
	}
	for _, text := range messages {
		if strings.TrimSpace(text) == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > 4000 {
			return webapp.BadRequest("Сообщение для производства должно содержать от 1 до 4000 символов.", nil)
		}
	}

	var value *string
	if err := tx.QueryRow(ctx, `
		SELECT value #>> '{}' FROM public.webapp_constants
		WHERE code = $1 AND value_type = 'string'
	`, productionMaxChatConstantCode).Scan(&value); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return productionMaxChatNotConfigured()
		}
		return fmt.Errorf("получение чата MAX для производства: %w", err)
	}
	chatID, err := parseProductionMaxChatID(value)
	if err != nil {
		return err
	}

	// Changed documents replace queued text. A call already in flight cannot be
	// recalled, but sender completion must never revive a superseded row.
	if _, err := tx.Exec(ctx, `
		UPDATE public.max_out_messages
		SET status = 'superseded', locked_at = NULL, next_attempt_at = NULL,
			error_str = 'Заменено новой версией документа.'
		WHERE metadata->>'source' = 'production_transfer'
			AND metadata->>'production_transfer_id' = $1::text
			AND metadata->>'version' <> $2::text
			AND status IN ('pending', 'sending', 'failed')
	`, strconv.Itoa(transferID), strconv.FormatInt(version, 10)); err != nil {
		return fmt.Errorf("замена предыдущих сообщений производства: %w", err)
	}

	sequenceKey := fmt.Sprintf("production_transfer:%d", transferID)
	for index, text := range messages {
		part := index + 1
		body, err := json.Marshal(map[string]any{"text": text})
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]any{
			"source":                 "production_transfer",
			"production_transfer_id": transferID,
			"version":                version,
			"part":                   part,
			"parts":                  len(messages),
		})
		if err != nil {
			return err
		}
		deliveryKey := fmt.Sprintf("%s:%d:%d", sequenceKey, version, part)
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.max_out_messages
				(max_chat_id, message, metadata, delivery_key, sequence_key, sequence_num)
			VALUES ($1, $2::jsonb, $3::jsonb, $4, $5, $6)
			ON CONFLICT (delivery_key) WHERE delivery_key IS NOT NULL DO NOTHING
		`, chatID, string(body), string(metadata), deliveryKey, sequenceKey, part); err != nil {
			return fmt.Errorf("постановка сообщения производства в очередь MAX: %w", err)
		}
	}
	return nil
}

func parseProductionMaxChatID(value *string) (int64, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return 0, productionMaxChatNotConfigured()
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(*value), 10, 64)
	if err != nil || chatID == 0 {
		return 0, webapp.BadRequest("В константе «Чат MAX для производства» укажите корректный chat_id целым числом, включая знак.", nil)
	}
	return chatID, nil
}

func productionMaxChatNotConfigured() error {
	return webapp.BadRequest("Не задана константа «Чат MAX для производства». Укажите chat_id общего чата MAX в настройках констант.", nil)
}

// Retry only failed messages of the currently saved version. Successful parts
// and superseded versions must never be resent by this action.
func retryProductionMessages(ctx context.Context, db ds.Querier, transferID int) (int64, error) {
	if transferID <= 0 {
		return 0, webapp.BadRequest("Некорректный номер документа производства.", nil)
	}
	result, err := db.Exec(ctx, `
		UPDATE public.max_out_messages AS message
		SET status = 'pending', attempt_count = 0, next_attempt_at = now(),
			locked_at = NULL, error_str = NULL
		WHERE message.metadata->>'source' = 'production_transfer'
			AND message.metadata->>'production_transfer_id' = $1::text
			AND message.metadata->>'version' = (
				SELECT version::text FROM public.production_transfers WHERE id = $2
			)
			AND message.status = 'failed'
	`, strconv.Itoa(transferID), transferID)
	if err != nil {
		return 0, fmt.Errorf("повторная отправка сообщений производства: %w", err)
	}
	if result.RowsAffected() > 0 {
		if _, err := db.Exec(ctx, `SELECT pg_notify('max_out_messages', $1)`, strconv.Itoa(transferID)); err != nil {
			return 0, err
		}
	}
	return result.RowsAffected(), nil
}

func cancelProductionMessages(ctx context.Context, db ds.Querier, transferID int) error {
	_, err := db.Exec(ctx, `
		UPDATE public.max_out_messages
		SET status = 'superseded', next_attempt_at = NULL, locked_at = NULL,
			error_str = 'Документ производства удалён.'
		WHERE metadata->>'source' = 'production_transfer'
			AND metadata->>'production_transfer_id' = $1::text
			AND status IN ('pending', 'sending', 'failed')
	`, strconv.Itoa(transferID))
	return err
}
