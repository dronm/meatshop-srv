package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/types"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

type NotificationTemplateRecipientService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewNotificationTemplateRecipientService(ctx webapp.ServiceContext) any {
	return &NotificationTemplateRecipientService{DB: ctx.DB, Session: ctx.Session, QueryID: ctx.QueryID}
}

func RegisterNotificationTemplateRecipientService() {
	webapp.MustRegisterService(
		"NotificationTemplateRecipient",
		&NotificationTemplateRecipientService{},
		NewNotificationTemplateRecipientService,
		webapp.WithCRUDNotifications(),
	)
}

func (s *NotificationTemplateRecipientService) Create(
	ctx context.Context,
	input modelbind.ModelInput[*models.NotificationTemplateRecipient],
) (map[string]any, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input.Model == nil || input.Model.TemplateID <= 0 || input.Model.UserID <= 0 {
		return nil, webapp.BadRequest("template_id and user_id should be positive", nil)
	}
	result, err := webapp.InsertModelInput(ctx, s.DB, input, nil)
	if err != nil {
		return nil, fmt.Errorf("insert notification template recipient: %w", err)
	}
	return result, nil
}

func (s *NotificationTemplateRecipientService) Update(
	ctx context.Context,
	input webapp.UpdateByKeysInput[*models.NotificationTemplateRecipientKey, *models.NotificationTemplateRecipient],
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if input.Keys == nil || input.Keys.ID <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("notification template recipient id is required", nil)
	}
	rowsAffected, err := webapp.UpdateModelInput(ctx, s.DB, input.Keys, input.Input, nil)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("update notification template recipient: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("notification template recipient not found", map[string]any{"id": input.Keys.ID})
	}
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *NotificationTemplateRecipientService) Delete(ctx context.Context, id int) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if id <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("notification template recipient id is required", nil)
	}
	rowsAffected, err := webapp.DeleteModel(ctx, s.DB, []types.DBModel{models.NotificationTemplateRecipientKey{ID: id}}, nil)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete notification template recipient: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("notification template recipient not found", map[string]any{"id": id})
	}
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *NotificationTemplateRecipientService) Detail(ctx context.Context, id int) (*models.NotificationTemplateRecipientDetail, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("notification template recipient id is required", nil)
	}
	result, err := webapp.FetchModel(ctx, s.DB, models.NotificationTemplateRecipientKey{ID: id}, &models.NotificationTemplateRecipientDetail{})
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("notification template recipient not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch notification template recipient: %w", err)
	}
	return result, nil
}

func (s *NotificationTemplateRecipientService) List(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.NotificationTemplateRecipientList], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.NotificationTemplateRecipientList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.NotificationTemplateRecipientList]{}, err
	}
	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.NotificationTemplateRecipientList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.NotificationTemplateRecipientList]{}, fmt.Errorf("fetch notification template recipient collection: %w", err)
	}
	return wmodels.CollectionResponse[*models.NotificationTemplateRecipientList]{Rows: rows, Agg: total}, nil
}

func (s *NotificationTemplateRecipientService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}
	return nil
}

func (s *NotificationTemplateRecipientService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}
	return nil
}
