package services

import (
	"context"
	"fmt"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

// LinesList returns the separate, one-row-per-item order projection.
func (s *OrderService) LinesList(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.OrderLineList], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.OrderLineList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.OrderLineList]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.OrderLineList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.OrderLineList]{}, fmt.Errorf(
			"fetch order line collection: %w",
			err,
		)
	}

	return wmodels.CollectionResponse[*models.OrderLineList]{
		Rows: rows,
		Agg:  total,
	}, nil
}
