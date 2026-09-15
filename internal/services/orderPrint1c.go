package services

import (
	"context"
	"errors"
	"strings"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func GetOrderPrint1C(
	ctx context.Context,
	db ds.Provider,
	client *integration.Client,
	id int,
) (integration.BinaryResponse, error) {
	if db == nil {
		return integration.BinaryResponse{}, webapp.Internal("database is not initialized", nil)
	}
	if client == nil {
		return integration.BinaryResponse{}, webapp.Internal("1c integration is not configured", nil)
	}
	if id <= 0 {
		return integration.BinaryResponse{}, webapp.BadRequest("order id should be positive", nil)
	}

	order, err := webapp.FetchModel(
		ctx,
		db,
		models.OrderKey{ID: id},
		&models.Order{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return integration.BinaryResponse{}, webapp.NotFound(
				"order not found",
				map[string]any{"id": id},
			)
		}
		return integration.BinaryResponse{}, webapp.Internal(
			"load order for 1c print form",
			map[string]any{"error": err.Error()},
		)
	}

	if order.Ref1C == nil || strings.TrimSpace(order.Ref1C.ID) == "" {
		return integration.BinaryResponse{}, webapp.BadRequest(
			"order has no 1c reference",
			map[string]any{"id": id},
		)
	}

	result, err := client.OrderPrintForm(ctx, strings.TrimSpace(order.Ref1C.ID))
	if err != nil {
		return integration.BinaryResponse{}, webapp.Internal(
			"1c order print form failed",
			map[string]any{"error": err.Error()},
		)
	}

	return result, nil
}
