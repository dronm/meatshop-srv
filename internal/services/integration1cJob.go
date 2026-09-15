package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

// Integration1CJobService exposes asynchronous 1C jobs as a read-only collection.
type Integration1CJobService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewIntegration1CJobService(ctx webapp.ServiceContext) any {
	return &Integration1CJobService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	}
}

func RegisterIntegration1CJobService() {
	webapp.MustRegisterService(
		"Integration1CJob",
		&Integration1CJobService{},
		NewIntegration1CJobService,
	)
}

func (s *Integration1CJobService) List(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.Integration1CJob], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.Integration1CJob]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.Integration1CJob]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(
		ctx,
		s.DB,
		&models.Integration1CJob{},
		params,
	)
	if err != nil {
		return wmodels.CollectionResponse[*models.Integration1CJob]{}, fmt.Errorf(
			"fetch integration 1c jobs: %w",
			err,
		)
	}

	return wmodels.CollectionResponse[*models.Integration1CJob]{
		Rows: rows,
		Agg:  total,
	}, nil
}

func (s *Integration1CJobService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}
	return nil
}

func (s *Integration1CJobService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}
	return nil
}
