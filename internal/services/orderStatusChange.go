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

const orderStatusChangeMaxOrders = 1000

// ChangeStatus changes only order headers. All selected orders and their MAX
// notifications commit together; item quantities and register actions stay intact.
func (s *OrderService) ChangeStatus(
	ctx context.Context,
	input models.OrderStatusChangeRequest,
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	orderIDs, err := orderStatusChangeIDs(input)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	var changed int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var err error
		changed, err = changeOrderStatuses(ctx, tx, orderIDs, input.StatusID)
		return err
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("не удалось изменить статусы заказов: %w", err)
	}
	return wmodels.RowsAffectedResponse{RowsAffected: changed}, nil
}

func orderStatusChangeIDs(input models.OrderStatusChangeRequest) ([]int, error) {
	if input.StatusID <= 0 {
		return nil, webapp.BadRequest("Не указан статус заказа.", nil)
	}
	if len(input.OrderIDs) == 0 {
		return nil, webapp.BadRequest("Выберите хотя бы один заказ.", nil)
	}
	if len(input.OrderIDs) > orderStatusChangeMaxOrders {
		return nil, webapp.BadRequest("Выбрано слишком много заказов.", map[string]any{"maximum": orderStatusChangeMaxOrders})
	}
	seen := make(map[int]struct{}, len(input.OrderIDs))
	ids := make([]int, 0, len(input.OrderIDs))
	for _, id := range input.OrderIDs {
		if id <= 0 {
			return nil, webapp.BadRequest("Некорректный идентификатор заказа.", nil)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	// A stable lock order avoids deadlocks between overlapping selections.
	sort.Ints(ids)
	return ids, nil
}

func changeOrderStatuses(ctx context.Context, tx ds.Querier, orderIDs []int, statusID int) (int64, error) {
	var existingStatusID int
	if err := tx.QueryRow(ctx, `
		SELECT id FROM public.order_statuses WHERE id = $1 FOR KEY SHARE
	`, statusID).Scan(&existingStatusID); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return 0, webapp.BadRequest("Статус заказа не найден.", map[string]any{"status_id": statusID})
		}
		return 0, err
	}
	if err := lockProductRegisterRecorders(ctx, tx, orderRecorderType, orderIDs...); err != nil {
		return 0, err
	}

	type orderState struct {
		id             int
		statusID       *int
		customerUserID *int
	}
	states := make([]orderState, 0, len(orderIDs))
	// Validate and lock the entire selection before changing its first order.
	for _, id := range orderIDs {
		state := orderState{id: id}
		if err := tx.QueryRow(ctx, `
			SELECT status_id, customer_user_id FROM public.orders WHERE id = $1 FOR UPDATE
		`, id).Scan(&state.statusID, &state.customerUserID); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return 0, webapp.NotFound("Заказ не найден.", map[string]any{"id": id})
			}
			return 0, err
		}
		states = append(states, state)
	}

	var changed int64
	for _, state := range states {
		if state.statusID != nil && *state.statusID == statusID {
			if err := recordManualOrderStatus(ctx, tx, state.id, &statusID); err != nil {
				return 0, err
			}
			continue
		}
		var version int64
		if err := tx.QueryRow(ctx, `
			UPDATE public.orders SET status_id = $2, version = version + 1
			WHERE id = $1 RETURNING version
		`, state.id, statusID).Scan(&version); err != nil {
			return 0, err
		}
		// Preserve the existing correction behavior and its version-aware 1C jobs.
		if err := syncOrder1CAfterStatusChange(ctx, tx, state.id, version); err != nil {
			return 0, err
		}
		if state.customerUserID != nil && *state.customerUserID > 0 {
			detail, err := fetchOrderDetail(ctx, tx, state.id)
			if err != nil {
				return 0, err
			}
			if err := enqueueOrderCustomerNotification(ctx, tx, state.customerUserID,
				orderNotificationDataFromDetail(detail, true, nil, nil)); err != nil {
				return 0, err
			}
		}
		changed++
	}
	if changed > 0 {
		// The grid already subscribes to Order.Update and reloads on that event.
		// PostgreSQL delivers this single compact notification only after commit.
		if _, err := tx.Exec(ctx, `SELECT pg_notify('Order.Update', $1)`,
			fmt.Sprintf(`{"result":{"rows_affected":%d}}`, changed)); err != nil {
			return 0, fmt.Errorf("не удалось уведомить об изменении статусов заказов: %w", err)
		}
	}
	return changed, nil
}
