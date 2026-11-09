package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

// ToggleLinesMercury sets all selected lines unless they are already all set.
func (s *OrderService) ToggleLinesMercury(
	ctx context.Context,
	input models.OrderLineIDsRequest,
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	itemIDs, err := validatedOrderLineIDs(input)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	var changed int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var err error
		changed, err = toggleOrderLinesMercury(ctx, tx, itemIDs)
		return err
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("toggle order line Mercury flags: %w", err)
	}
	return wmodels.RowsAffectedResponse{RowsAffected: changed}, nil
}

func toggleOrderLinesMercury(ctx context.Context, tx ds.Querier, itemIDs []int) (int64, error) {
	// Document writers lock the order before its items. Use the same order
	// for bulk selections, then read current flags after any wait for a lock.
	if _, err := tx.Exec(ctx, `
		SELECT id FROM public.orders
		WHERE id IN (
			SELECT order_id FROM public.order_items WHERE id = ANY($1::integer[])
		)
		ORDER BY id FOR UPDATE
	`, itemIDs); err != nil {
		return 0, err
	}

	var count int64
	var allMarked bool
	if err := tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(bool_and(mercury), false)
		FROM (
			SELECT mercury FROM public.order_items
			WHERE id = ANY($1::integer[])
			ORDER BY id FOR UPDATE
		) AS selected_items
	`, itemIDs).Scan(&count, &allMarked); err != nil {
		return 0, err
	}
	if count != int64(len(itemIDs)) {
		return 0, webapp.NotFound("one or more selected order items were not found", nil)
	}

	result, err := tx.Exec(ctx, `
		UPDATE public.order_items SET mercury = $2
		WHERE id = ANY($1::integer[]) AND mercury IS DISTINCT FROM $2
	`, itemIDs, !allMarked)
	if err != nil {
		return 0, err
	}
	changed := result.RowsAffected()
	if _, err := tx.Exec(ctx, `SELECT pg_notify('Order.Update', $1)`,
		fmt.Sprintf(`{"result":{"rows_affected":%d}}`, changed)); err != nil {
		return 0, err
	}
	return changed, nil
}
