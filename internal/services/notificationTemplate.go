package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/types"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

type NotificationTemplateService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewNotificationTemplateService(ctx webapp.ServiceContext) any {
	return &NotificationTemplateService{DB: ctx.DB, Session: ctx.Session, QueryID: ctx.QueryID}
}

func RegisterNotificationTemplateService() {
	webapp.MustRegisterService(
		"NotificationTemplate",
		&NotificationTemplateService{},
		NewNotificationTemplateService,
		webapp.WithCRUDNotifications(),
	)
}

func (s *NotificationTemplateService) Create(
	ctx context.Context,
	input modelbind.ModelInput[*models.NotificationTemplate],
) (map[string]any, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input.Model == nil {
		return nil, webapp.BadRequest("notification template input is required", nil)
	}
	if err := normalizeNotificationTemplate(input.Model); err != nil {
		return nil, err
	}
	result, err := webapp.InsertModelInput(ctx, s.DB, input, nil)
	if err != nil {
		return nil, fmt.Errorf("insert notification template: %w", err)
	}
	return result, nil
}

func (s *NotificationTemplateService) Update(
	ctx context.Context,
	input webapp.UpdateByKeysInput[*models.NotificationTemplateKey, *models.NotificationTemplate],
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if input.Keys == nil || input.Keys.ID <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("notification template id is required", nil)
	}
	if input.Input.Model != nil {
		normalizeNotificationTemplateUpdate(input.Input.Model)
		if input.Input.Model.BodyTemplate != "" {
			if _, err := template.New("notification").Option("missingkey=error").Parse(input.Input.Model.BodyTemplate); err != nil {
				return wmodels.RowsAffectedResponse{}, webapp.BadRequest("invalid notification body_template", map[string]any{"error": err.Error()})
			}
		}
	}
	rowsAffected, err := webapp.UpdateModelInput(ctx, s.DB, input.Keys, input.Input, nil)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("update notification template: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("notification template not found", map[string]any{"id": input.Keys.ID})
	}
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *NotificationTemplateService) Delete(ctx context.Context, id int) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if id <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("notification template id is required", nil)
	}
	rowsAffected, err := webapp.DeleteModel(ctx, s.DB, []types.DBModel{models.NotificationTemplateKey{ID: id}}, nil)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete notification template: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("notification template not found", map[string]any{"id": id})
	}
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *NotificationTemplateService) Detail(ctx context.Context, id int) (*models.NotificationTemplate, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("notification template id is required", nil)
	}
	result, err := webapp.FetchModel(ctx, s.DB, models.NotificationTemplateKey{ID: id}, &models.NotificationTemplate{})
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("notification template not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch notification template: %w", err)
	}
	return result, nil
}

func (s *NotificationTemplateService) List(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.NotificationTemplate], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.NotificationTemplate]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.NotificationTemplate]{}, err
	}
	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.NotificationTemplate{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.NotificationTemplate]{}, fmt.Errorf("fetch notification template collection: %w", err)
	}
	return wmodels.CollectionResponse[*models.NotificationTemplate]{Rows: rows, Agg: total}, nil
}

func (s *NotificationTemplateService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}
	return nil
}

func (s *NotificationTemplateService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}
	return nil
}

func normalizeNotificationTemplate(model *models.NotificationTemplate) error {
	model.Code = strings.TrimSpace(model.Code)
	model.Event = strings.TrimSpace(model.Event)
	model.BodyTemplate = strings.TrimSpace(model.BodyTemplate)
	if model.Code == "" {
		return webapp.BadRequest("notification template code is required", nil)
	}
	if model.Event == "" {
		return webapp.BadRequest("notification template event is required", nil)
	}
	if model.BodyTemplate == "" {
		return webapp.BadRequest("notification template body_template is required", nil)
	}
	if _, err := template.New(model.Code).Option("missingkey=error").Parse(model.BodyTemplate); err != nil {
		return webapp.BadRequest("invalid notification body_template", map[string]any{"error": err.Error()})
	}
	return nil
}

func normalizeNotificationTemplateUpdate(model *models.NotificationTemplate) {
	model.Code = strings.TrimSpace(model.Code)
	model.Event = strings.TrimSpace(model.Event)
	model.BodyTemplate = strings.TrimSpace(model.BodyTemplate)
}
