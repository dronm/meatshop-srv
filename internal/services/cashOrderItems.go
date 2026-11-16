package services

import (
	"context"
	"fmt"
	"math"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

func normalizeCashOrderItem(item *models.OrderDocumentItem, index int) error {
	if math.IsNaN(item.Quant) || math.IsInf(item.Quant, 0) || item.Quant <= 0 {
		return invalidOrderDocumentItem(index, "quant should be greater than zero for a cash order")
	}
	item.QuantRequired = item.Quant
	item.Price = trimOptionalString(item.Price)
	item.Amount = trimOptionalString(item.Amount)
	for _, field := range []struct {
		name  string
		value *string
		scale int
	}{
		{"price", item.Price, 6},
		{"amount", item.Amount, 2},
	} {
		if field.value != nil {
			if err := validateOrder1CDecimal(*field.value, 13, field.scale); err != nil {
				return invalidOrderDocumentItem(index, fmt.Sprintf("%s: %v", field.name, err))
			}
		}
	}
	return nil
}

// Manual prices belong to cash orders only. Omitted values keep an existing
// price/amount; newly inserted rows default to zero. VAT remains server-owned.
func syncCashOrderItemPrices(ctx context.Context, tx ds.Querier, document *models.OrderDocument) error {
	if !document.CashPayed {
		return nil
	}
	for _, item := range document.Items {
		if _, err := tx.Exec(ctx, `
			UPDATE public.order_items
			SET price = COALESCE($3::text::numeric, price, 0),
				amount = COALESCE($4::text::numeric, amount, 0)
			WHERE id = $1 AND order_id = $2
		`, item.ID, document.ID, item.Price, item.Amount); err != nil {
			return fmt.Errorf("save cash order item %d prices: %w", item.ID, err)
		}
	}
	return nil
}
