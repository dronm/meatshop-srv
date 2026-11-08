package services

import (
	"context"
	"sort"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

type orderItemWeightState struct {
	ID                         int
	LineNum                    int
	ProductID                  int
	MeasureUnitID              int
	QuantRequired              float64
	Quant                      float64
	WeightConfirmed            bool
	RequiresWeightConfirmation bool
}

func adminOrderItemWeightConfirmed(item *models.OrderDocumentItem, old *orderItemWeightState, requires bool) bool {
	if !requires {
		return true
	}
	if item.WeightConfirmed != nil && *item.WeightConfirmed {
		return true
	}
	// Copies and newly selected products/units require a fresh weighing decision.
	// A quantity copied from another line is not a weighing action on this line.
	if old == nil || old.ProductID != item.ProductID || old.MeasureUnitID != item.MeasureUnitID {
		return false
	}
	if old.Quant != item.Quant {
		return true
	}
	if old.QuantRequired != item.QuantRequired {
		return false
	}
	if item.WeightConfirmed != nil {
		return *item.WeightConfirmed
	}
	return old.WeightConfirmed
}

func loadOrderItemWeightStates(ctx context.Context, tx ds.Querier, orderID int) (map[int]orderItemWeightState, error) {
	rows, err := tx.Query(ctx, `
		SELECT item.id, item.line_num, item.product_id, item.measure_unit_id,
			item.quant_required::double precision, item.quant::double precision,
			item.weight_confirmed, COALESCE(product.requires_weight_confirmation, false)
		FROM public.order_items AS item
		LEFT JOIN public.products AS product ON product.id = item.product_id
		WHERE item.order_id = $1
		ORDER BY item.line_num, item.id
		FOR UPDATE OF item
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int]orderItemWeightState)
	for rows.Next() {
		var item orderItemWeightState
		if err := rows.Scan(&item.ID, &item.LineNum, &item.ProductID, &item.MeasureUnitID,
			&item.QuantRequired, &item.Quant, &item.WeightConfirmed, &item.RequiresWeightConfirmation); err != nil {
			return nil, err
		}
		result[item.ID] = item
	}
	return result, rows.Err()
}

func loadOrderProductWeightRequirements(ctx context.Context, tx ds.Querier, items []*models.OrderDocumentItem) (map[int]bool, error) {
	ids := make([]int, 0, len(items))
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item.ProductID]; !exists {
			ids = append(ids, item.ProductID)
			seen[item.ProductID] = struct{}{}
		}
	}
	sort.Ints(ids)
	rows, err := tx.Query(ctx, `
		SELECT id, requires_weight_confirmation FROM public.products
		WHERE id = ANY($1::integer[])
		ORDER BY id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int]bool, len(ids))
	for rows.Next() {
		var id int
		var requires bool
		if err := rows.Scan(&id, &requires); err != nil {
			return nil, err
		}
		result[id] = requires
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, exists := result[id]; !exists {
			return nil, webapp.BadRequest("order product not found", map[string]any{"product_id": id})
		}
	}
	return result, nil
}
