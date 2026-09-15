package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
)

type MaxNotificationService struct {
	DB      ds.Provider
	Session session.Session
}

func NewMaxNotificationService(ctx webapp.ServiceContext) any {
	return &MaxNotificationService{
		DB:      ctx.DB,
		Session: ctx.Session,
	}
}

func RegisterMaxNotificationService() {
	webapp.MustRegisterService(
		"MaxNotification",
		&MaxNotificationService{},
		NewMaxNotificationService,
	)
}

func (s *MaxNotificationService) Send(
	ctx context.Context,
	input models.MaxNotificationSendRequest,
) (*models.MaxNotificationSendResponse, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	if s.DB == nil {
		return nil, webapp.Internal("database is not initialized", nil)
	}
	if input.MaxUserID <= 0 {
		return nil, webapp.BadRequest("max_user_id should be positive", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for MAX notification: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := enqueueMaxOutMessage(ctx, poolConn.Conn(), input.MaxUserID, input.Text, input.Metadata)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("active MAX user not found", map[string]any{
				"max_user_id": input.MaxUserID,
			})
		}
		return nil, fmt.Errorf("queue MAX notification: %w", err)
	}
	return result, nil
}

// enqueueMaxOutMessage inserts a durable MAX message using the supplied database
// handle. Passing an existing transaction keeps domain changes and notification
// enqueueing atomic. maxUserID is the external MAX platform user ID.
func enqueueMaxOutMessage(
	ctx context.Context,
	db ds.Querier,
	maxUserID int64,
	text string,
	metadata map[string]any,
) (*models.MaxNotificationSendResponse, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, webapp.BadRequest("text is required", nil)
	}
	if len([]rune(text)) > 4000 {
		return nil, webapp.BadRequest("text is too long", map[string]any{"max_length": 4000})
	}

	message, err := json.Marshal(map[string]any{"text": text})
	if err != nil {
		return nil, fmt.Errorf("encode MAX notification: %w", err)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("encode MAX notification metadata: %w", err)
	}

	result := &models.MaxNotificationSendResponse{}
	err = db.QueryRow(ctx, `
		INSERT INTO public.max_out_messages (
			max_user_id,
			message,
			metadata
		)
		SELECT
			u.max_user_id,
			$2::jsonb,
			$3::jsonb
		FROM public.max_users AS u
		WHERE u.max_user_id = $1
			AND u.is_active
		RETURNING id, created_at
	`, maxUserID, string(message), string(metadataJSON)).Scan(&result.ID, &result.CreatedAt)
	if err != nil {
		return nil, err
	}
	return result, nil
}
