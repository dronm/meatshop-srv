package services

import (
	"context"
	"fmt"
	"strconv"

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

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.OrderLineList{}, orderLinesListParams(params))
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

func orderLinesListParams(params modelbind.CollectionParams) modelbind.CollectionParams {
	for _, filter := range params.Filter {
		for field, condition := range filter.Fields {
			switch field {
			case "customer->'keys'->>'id'", "customer_sale_place->'keys'->>'id'", "product->'keys'->>'id'":
				// JSON ->> returns text. Reference pickers send numeric IDs;
				// pgx needs text parameters for these exact comparisons.
				switch value := condition.Value.(type) {
				case float64:
					condition.Value = strconv.FormatFloat(value, 'f', -1, 64)
					filter.Fields[field] = condition
				case int, int64:
					condition.Value = fmt.Sprint(condition.Value)
					filter.Fields[field] = condition
				}
			}
		}
	}

	return params
}
