package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

// SetLinesWeightConfirmed is an idempotent preparation command. Normal products
// and removed lines are ignored when the selection contains several item types.
func (s *OrderService) SetLinesWeightConfirmed(
	ctx context.Context,
	input models.OrderLinesWeightConfirmationRequest,
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	itemIDs, err := weightConfirmationItemIDs(input)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	var changed int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var err error
		changed, err = setOrderLinesWeightConfirmed(ctx, tx, itemIDs, *input.WeightConfirmed)
		return err
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("Не удалось изменить подтверждение веса позиций заказа: %w", err)
	}
	return wmodels.RowsAffectedResponse{RowsAffected: changed}, nil
}

func weightConfirmationItemIDs(input models.OrderLinesWeightConfirmationRequest) ([]int, error) {
	if input.WeightConfirmed == nil {
		return nil, webapp.BadRequest("Укажите, нужно ли подтвердить вес выбранных позиций.", nil)
	}
	itemIDs, err := validatedOrderLineIDs(models.OrderLineIDsRequest{ItemIDs: input.ItemIDs})
	if err != nil {
		return nil, webapp.BadRequest(
			"Выберите от 1 до 1000 позиций заказа с положительными идентификаторами.", nil,
		)
	}
	return itemIDs, nil
}

func setOrderLinesWeightConfirmed(ctx context.Context, tx ds.Querier, itemIDs []int, confirmed bool) (int64, error) {
	// Match aggregate writers' parent-before-item lock order. Stable ordering also
	// protects overlapping multi-order selections from locking each other in reverse.
	if _, err := tx.Exec(ctx, `
		SELECT id FROM public.orders
		WHERE id IN (
			SELECT order_id FROM public.order_items WHERE id = ANY($1::integer[])
		)
		ORDER BY id FOR UPDATE
	`, itemIDs); err != nil {
		return 0, err
	}

	rows, err := tx.Query(ctx, `
		SELECT item.id, item.quant::double precision,
			COALESCE(product.requires_weight_confirmation, false), item.weight_confirmed
		FROM public.order_items AS item
		LEFT JOIN public.products AS product ON product.id = item.product_id
		WHERE item.id = ANY($1::integer[])
		ORDER BY item.id FOR UPDATE OF item
	`, itemIDs)
	if err != nil {
		return 0, err
	}
	selectedCount := 0
	changedIDs := make([]int, 0, len(itemIDs))
	for rows.Next() {
		var id int
		var quant float64
		var required, previous bool
		if err := rows.Scan(&id, &quant, &required, &previous); err != nil {
			rows.Close()
			return 0, err
		}
		selectedCount++
		if quant > 0 && required && previous != confirmed {
			changedIDs = append(changedIDs, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	// Validate the entire selection before writing even when only some products
	// need confirmation. A stale selection must not silently update a partial set.
	if selectedCount != len(itemIDs) {
		return 0, webapp.NotFound("Одна или несколько выбранных позиций заказа не найдены. Обновите список.", nil)
	}
	if len(changedIDs) == 0 {
		return 0, nil
	}

	result, err := tx.Exec(ctx, `
		UPDATE public.order_items AS item SET weight_confirmed = $2
		WHERE item.id = ANY($1::integer[]) AND item.quant > 0
			AND item.weight_confirmed IS DISTINCT FROM $2
			AND EXISTS (
				SELECT 1 FROM public.products AS product
				WHERE product.id = item.product_id AND product.requires_weight_confirmation
			)
	`, changedIDs, confirmed)
	if err != nil {
		return 0, err
	}
	changed := result.RowsAffected()
	if changed != int64(len(changedIDs)) {
		return 0, webapp.Conflict("Выбранные позиции заказа изменились. Обновите список и повторите действие.", nil)
	}

	// As with Mercury, preparation-only flags do not change the order version or
	// invalidate pending 1C jobs/prices. Aggregate forms omit unchanged flags, so
	// an unrelated save reads and preserves the latest confirmation under locks.
	if _, err := tx.Exec(ctx, `SELECT pg_notify('Order.Update', $1)`,
		fmt.Sprintf(`{"result":{"rows_affected":%d}}`, changed)); err != nil {
		return 0, err
	}
	return changed, nil
}
