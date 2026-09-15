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

type CustomerUserService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}

func NewCustomerUserService(ctx webapp.ServiceContext) any {
	return &CustomerUserService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
	}
}

func RegisterCustomerUserService() {
	webapp.MustRegisterService(
		"CustomerUser",
		&CustomerUserService{},
		NewCustomerUserService,
	)
}

func (s *CustomerUserService) List(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.CustomerUserList], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.CustomerUserList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.CustomerUserList]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.CustomerUserList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.CustomerUserList]{}, fmt.Errorf(
			"fetch customer user collection: %w",
			err,
		)
	}

	return wmodels.CollectionResponse[*models.CustomerUserList]{
		Rows: rows,
		Agg:  total,
	}, nil
}

func (s *CustomerUserService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}

	return nil
}

func (s *CustomerUserService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}

	return nil
}
