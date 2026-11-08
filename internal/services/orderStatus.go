package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

const (
	orderStatusNew        = "new"
	orderStatusProcessing = "processing"
)

// Customers may correct submitted orders until the manager starts preparing them.
// Keep legacy new orders editable as well.
func canCustomerChangeOrder(statusCode string) bool {
	return statusCode == orderStatusNew || statusCode == orderStatusProcessing
}

func defaultAdminOrderStatus(ctx context.Context, tx ds.Querier, document *models.OrderDocument) error {
	if document.StatusID != nil {
		return nil
	}
	statusID, err := orderStatusID(ctx, tx, orderStatusNew)
	if err != nil {
		return err
	}
	document.StatusID = &statusID
	return nil
}

func orderStatusID(ctx context.Context, tx ds.Querier, code string) (int, error) {
	var id int
	if err := tx.QueryRow(ctx, `SELECT id FROM public.order_statuses WHERE code = $1`, code).Scan(&id); err != nil {
		return 0, fmt.Errorf("load order status %q: %w", code, err)
	}
	return id, nil
}
