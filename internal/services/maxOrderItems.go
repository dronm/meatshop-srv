package services

import (
	"context"
	"errors"
	"sort"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

type maxOrderProductState struct {
	MeasureUnitID              int
	RequiresWeightConfirmation bool
}

// syncMaxOrderItems keeps stable line identities when a customer changes the
// schedule, comment or item order. MAX never supplies a weight confirmation.
func syncMaxOrderItems(ctx context.Context, tx ds.Querier, orderID int, inputs []models.MaxOrderItemInput) error {
	existing, err := loadOrderItemWeightStates(ctx, tx, orderID)
	if err != nil {
		return err
	}
	products := make(map[int]maxOrderProductState, len(inputs))
	for _, input := range inputs {
		if _, exists := products[input.ProductID]; exists {
			continue
		}
		var product maxOrderProductState
		if err := tx.QueryRow(ctx, `
			SELECT measure_unit_id, requires_weight_confirmation
			FROM public.products
			WHERE id = $1 AND is_active AND NOT is_group AND measure_unit_id IS NOT NULL
		`, input.ProductID).Scan(&product.MeasureUnitID, &product.RequiresWeightConfirmation); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.BadRequest("product is not available for ordering", map[string]any{"product_id": input.ProductID})
			}
			return err
		}
		products[input.ProductID] = product
	}
	items, err := prepareMaxOrderItems(inputs, products, existing)
	if err != nil {
		return err
	}
	return syncResolvedOrderItems(ctx, tx, orderID, items, existing)
}

func prepareMaxOrderItems(
	inputs []models.MaxOrderItemInput,
	products map[int]maxOrderProductState,
	existing map[int]orderItemWeightState,
) ([]*models.OrderDocumentItem, error) {
	existingIDs := make([]int, 0, len(existing))
	for id := range existing {
		existingIDs = append(existingIDs, id)
	}
	sort.Slice(existingIDs, func(i, j int) bool {
		left, right := existing[existingIDs[i]], existing[existingIDs[j]]
		if left.LineNum == right.LineNum {
			return existingIDs[i] < existingIDs[j]
		}
		return left.LineNum < right.LineNum
	})

	items := make([]*models.OrderDocumentItem, len(inputs))
	for index, input := range inputs {
		product, exists := products[input.ProductID]
		if !exists {
			return nil, webapp.BadRequest("product is not available for ordering", map[string]any{"product_id": input.ProductID})
		}
		confirmed := !product.RequiresWeightConfirmation
		items[index] = &models.OrderDocumentItem{
			LineNum: index + 1, ProductID: input.ProductID, MeasureUnitID: product.MeasureUnitID,
			QuantRequired: input.QuantRequired, Quant: input.QuantRequired, WeightConfirmed: &confirmed,
		}
	}

	matched := make(map[int]struct{}, len(existing))
	// Reserve exact occurrences across the entire request first. A changed
	// quantity must not steal another occurrence's confirmed weight or identity.
	for _, item := range items {
		matchedID := 0
		for _, id := range existingIDs {
			if _, used := matched[id]; used {
				continue
			}
			previous := existing[id]
			if previous.ProductID != item.ProductID || previous.MeasureUnitID != item.MeasureUnitID ||
				previous.QuantRequired != item.QuantRequired {
				continue
			}
			if matchedID != 0 {
				return nil, ambiguousMaxOrderItem(item)
			}
			matchedID = id
		}
		if matchedID != 0 {
			previous := existing[matchedID]
			confirmed := previous.WeightConfirmed || !products[item.ProductID].RequiresWeightConfirmation
			item.ID, item.Quant, item.WeightConfirmed = matchedID, previous.Quant, &confirmed
			matched[matchedID] = struct{}{}
		}
	}

	// A remaining occurrence of the same product/unit may reuse its identity,
	// but its changed request starts again with an unconfirmed estimated weight.
	for _, item := range items {
		if item.ID != 0 {
			continue
		}
		matchedID := 0
		for _, id := range existingIDs {
			if _, used := matched[id]; used {
				continue
			}
			previous := existing[id]
			if previous.ProductID == item.ProductID && previous.MeasureUnitID == item.MeasureUnitID {
				if matchedID != 0 {
					return nil, ambiguousMaxOrderItem(item)
				}
				matchedID = id
			}
		}
		if matchedID != 0 {
			item.ID = matchedID
			matched[matchedID] = struct{}{}
		}
	}
	return items, nil
}

func ambiguousMaxOrderItem(item *models.OrderDocumentItem) error {
	return webapp.Conflict("order items cannot be matched unambiguously; contact the manager", map[string]any{
		"product_id": item.ProductID, "line_num": item.LineNum,
	})
}
