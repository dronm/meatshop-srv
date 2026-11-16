package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	"github.com/dronm/webapp"
)

const productionStatusRecorderType = "production_transfer"

// The caller locks product recorders and orders in their normal stable order.
// These helpers belong to the surrounding production-document transaction.
func addProductionOrderStatus(ctx context.Context, tx ds.Querier, orderID, transferID int) error {
	if orderID <= 0 || transferID <= 0 {
		return webapp.BadRequest("Не указан заказ или документ передачи в производство.", nil)
	}
	statusID, err := orderStatusID(ctx, tx, "accepted")
	if err != nil {
		return fmt.Errorf("не удалось получить статус «Готовим»: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.order_status_assignments(order_id, status_id, recorder)
		VALUES ($1, $2, jsonb_build_object('document_type', $3::text, 'document_id', $4::integer))
		ON CONFLICT (order_id, recorder) WHERE is_active AND recorder IS NOT NULL DO NOTHING
	`, orderID, statusID, productionStatusRecorderType, transferID); err != nil {
		return fmt.Errorf("не удалось записать статус передачи в производство: %w", err)
	}
	return recalculateOrderStatusAssignments(ctx, tx, orderID)
}

func removeProductionOrderStatus(ctx context.Context, tx ds.Querier, orderID, transferID int) error {
	if orderID <= 0 || transferID <= 0 {
		return webapp.BadRequest("Не указан заказ или документ передачи в производство.", nil)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE public.order_status_assignments SET is_active = false
		WHERE order_id = $1 AND is_active
			AND recorder = jsonb_build_object('document_type', $2::text, 'document_id', $3::integer)
	`, orderID, productionStatusRecorderType, transferID); err != nil {
		return fmt.Errorf("не удалось отменить статус документа передачи в производство: %w", err)
	}
	return recalculateOrderStatusAssignments(ctx, tx, orderID)
}

// Explicitly assigning the current status is a manual ownership change, even
// though there is no numerical status change for the automatic orders trigger.
func recordManualOrderStatus(ctx context.Context, tx ds.Querier, orderID int, statusID *int) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.order_status_assignments(order_id, status_id)
		VALUES ($1, $2)
	`, orderID, statusID); err != nil {
		return fmt.Errorf("не удалось записать ручное назначение статуса заказа: %w", err)
	}
	return nil
}

func recalculateOrderStatusAssignments(ctx context.Context, tx ds.Querier, orderID int) error {
	var changed bool
	var version int64
	var customerUserID *int
	if err := tx.QueryRow(ctx, `
		SELECT status_changed, order_version, customer_user_id
		FROM public.order_status_assignments_recalculate($1)
	`, orderID).Scan(&changed, &version, &customerUserID); err != nil {
		return fmt.Errorf("не удалось восстановить действующий статус заказа: %w", err)
	}
	if !changed {
		return nil
	}
	if err := syncOrder1CAfterStatusChange(ctx, tx, orderID, version); err != nil {
		return err
	}
	if customerUserID != nil && *customerUserID > 0 {
		detail, err := fetchOrderDetail(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if err := enqueueOrderCustomerNotification(ctx, tx, customerUserID,
			orderNotificationDataFromDetail(detail, true, nil, nil)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `SELECT pg_notify('Order.Update', $1)`,
		fmt.Sprintf(`{"result":{"id":%d}}`, orderID)); err != nil {
		return fmt.Errorf("не удалось уведомить об изменении статуса заказа: %w", err)
	}
	return nil
}
