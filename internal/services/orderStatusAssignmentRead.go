package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

func (s *OrderStatusAssignmentService) Detail(ctx context.Context, id int64) (*models.OrderStatusAssignment, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("Не указан идентификатор записи статуса заказа.", nil)
	}
	result, err := webapp.FetchModel(ctx, s.DB, models.OrderStatusAssignmentKey{ID: id}, &models.OrderStatusAssignment{})
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("Запись статуса заказа не найдена.", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("не удалось получить запись статуса заказа: %w", err)
	}
	return result, nil
}

func (s *OrderStatusAssignmentService) List(ctx context.Context, params modelbind.CollectionParams) (wmodels.CollectionResponse[*models.OrderStatusAssignmentList], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.OrderStatusAssignmentList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.OrderStatusAssignmentList]{}, err
	}
	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.OrderStatusAssignmentList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.OrderStatusAssignmentList]{}, fmt.Errorf("не удалось получить историю статусов заказа: %w", err)
	}
	return wmodels.CollectionResponse[*models.OrderStatusAssignmentList]{Rows: rows, Agg: total}, nil
}
