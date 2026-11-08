package services

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const orderLinesRemoveMaxItems = 1000

// RemoveLines removes selected items from preparation by setting their confirmed
// quantity to zero. Requested quantities, item IDs and parent orders are retained.
func (s *OrderService) RemoveLines(
	ctx context.Context,
	input models.OrderLinesRemoveRequest,
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	itemIDs, err := orderLinesRemoveIDs(input)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	var changed int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var err error
		changed, err = removeOrderLines(ctx, tx, itemIDs)
		return err
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("remove order lines: %w", err)
	}
	return wmodels.RowsAffectedResponse{RowsAffected: changed}, nil
}

func orderLinesRemoveIDs(input models.OrderLinesRemoveRequest) ([]int, error) {
	if len(input.ItemIDs) == 0 {
		return nil, webapp.BadRequest("at least one order item should be selected", nil)
	}
	if len(input.ItemIDs) > orderLinesRemoveMaxItems {
		return nil, webapp.BadRequest("too many order item ids", map[string]any{"maximum": orderLinesRemoveMaxItems})
	}
	seen := make(map[int]struct{}, len(input.ItemIDs))
	ids := make([]int, 0, len(input.ItemIDs))
	for _, id := range input.ItemIDs {
		if id <= 0 {
			return nil, webapp.BadRequest("order item ids should be positive", nil)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids, nil
}

func removeOrderLines(ctx context.Context, tx ds.Querier, itemIDs []int) (int64, error) {
	// Discover parents without item locks: all document writers lock the parent
	// recorder/order before its items. Revalidate item membership after those locks.
	rows, err := tx.Query(ctx, `
		SELECT id, order_id FROM public.order_items
		WHERE id = ANY($1::integer[])
		ORDER BY order_id, id
	`, itemIDs)
	if err != nil {
		return 0, err
	}
	parents := make(map[int]int, len(itemIDs))
	orderIDs := make([]int, 0)
	seenOrders := make(map[int]struct{})
	for rows.Next() {
		var itemID, orderID int
		if err := rows.Scan(&itemID, &orderID); err != nil {
			rows.Close()
			return 0, err
		}
		parents[itemID] = orderID
		if _, exists := seenOrders[orderID]; !exists {
			seenOrders[orderID] = struct{}{}
			orderIDs = append(orderIDs, orderID)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, itemID := range itemIDs {
		if _, exists := parents[itemID]; !exists {
			return 0, webapp.NotFound("order item not found", map[string]any{"item_id": itemID})
		}
	}
	sort.Ints(orderIDs)
	if err := lockProductRegisterRecorders(ctx, tx, orderRecorderType, orderIDs...); err != nil {
		return 0, err
	}

	type orderState struct {
		id             int
		customerUserID *int
		changedItemIDs []int
	}
	states := make([]orderState, 0, len(orderIDs))
	// Validate the complete selection before its first write, including rows
	// replaced by a customer or removed in the editor while acquiring locks.
	for _, orderID := range orderIDs {
		state := orderState{id: orderID}
		if err := tx.QueryRow(ctx, `
			SELECT customer_user_id FROM public.orders WHERE id = $1 FOR UPDATE
		`, orderID).Scan(&state.customerUserID); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return 0, webapp.NotFound("order not found", map[string]any{"id": orderID})
			}
			return 0, err
		}
		snapshots, err := loadOrderItemSnapshots(ctx, tx, orderID)
		if err != nil {
			return 0, err
		}
		for _, itemID := range itemIDs {
			if parents[itemID] != orderID {
				continue
			}
			item, exists := snapshots[itemID]
			if !exists {
				return 0, webapp.Conflict("selected order item was changed or removed", map[string]any{"item_id": itemID})
			}
			if item.Quant > 0 {
				state.changedItemIDs = append(state.changedItemIDs, itemID)
			}
		}
		states = append(states, state)
	}

	var changed int64
	for _, state := range states {
		if len(state.changedItemIDs) == 0 {
			continue
		}
		result, err := tx.Exec(ctx, `
			UPDATE public.order_items SET quant = 0
			WHERE order_id = $1 AND id = ANY($2::integer[]) AND quant > 0
		`, state.id, state.changedItemIDs)
		if err != nil {
			return 0, err
		}
		if result.RowsAffected() != int64(len(state.changedItemIDs)) {
			return 0, webapp.Conflict("selected order items were changed", map[string]any{"order_id": state.id})
		}
		var version int64
		if err := tx.QueryRow(ctx, `
			UPDATE public.orders SET version = version + 1 WHERE id = $1 RETURNING version
		`, state.id).Scan(&version); err != nil {
			return 0, err
		}
		if err := rebuildProductRegisterActions(ctx, tx, orderRecorderType, state.id); err != nil {
			return 0, err
		}
		if err := syncOrder1CAfterCorrection(ctx, tx, state.id, version); err != nil {
			return 0, err
		}
		if state.customerUserID != nil && *state.customerUserID > 0 {
			detail, err := fetchOrderDetail(ctx, tx, state.id)
			if err != nil {
				return 0, err
			}
			changedIDs := make(map[int]struct{}, len(state.changedItemIDs))
			for _, itemID := range state.changedItemIDs {
				changedIDs[itemID] = struct{}{}
			}
			if err := enqueueOrderCustomerNotification(ctx, tx, state.customerUserID,
				orderNotificationDataFromDetail(detail, false, changedIDs, nil)); err != nil {
				return 0, err
			}
		}
		changed += result.RowsAffected()
	}
	if changed > 0 {
		if _, err := tx.Exec(ctx, `SELECT pg_notify('Order.Update', $1)`,
			fmt.Sprintf(`{"result":{"rows_affected":%d}}`, changed)); err != nil {
			return 0, err
		}
	}
	return changed, nil
}
