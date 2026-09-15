package services

import (
	"context"
	"fmt"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/webapp"
)

func validateOrderIDBatch(orderIDs []int) error {
	if err := integration.ValidateOrderIDs(orderIDs); err != nil {
		return webapp.BadRequest(err.Error(), nil)
	}
	return nil
}

func validateOrdersFor1CAction(
	ctx context.Context,
	db ds.Provider,
	orderIDs []int,
) error {
	poolConn, connID, err := db.GetPrimary(ctx)
	if err != nil {
		return webapp.Internal(
			"get primary connection for 1c order action",
			map[string]any{"error": err.Error()},
		)
	}
	defer db.Release(poolConn, connID)

	return validateOrdersFor1CActionQuery(
		ctx,
		poolConn.Conn(),
		orderIDs,
		false,
	)
}

func validateOrdersFor1CActionQuery(
	ctx context.Context,
	db ds.Querier,
	orderIDs []int,
	lockRows bool,
) error {
	query := `
		SELECT
			id,
			NULLIF(btrim(ref_1c->>'id'), '')
		FROM public.orders
		WHERE id = ANY($1)
		ORDER BY id
	`
	if lockRows {
		query += " FOR UPDATE"
	}

	rows, err := db.Query(ctx, query, orderIDs)
	if err != nil {
		return fmt.Errorf("load orders for 1c action: %w", err)
	}
	defer rows.Close()

	type orderState struct {
		orderRefID *string
	}
	states := make(map[int]orderState, len(orderIDs))
	for rows.Next() {
		var id int
		var state orderState
		if err := rows.Scan(&id, &state.orderRefID); err != nil {
			return fmt.Errorf("scan order for 1c action: %w", err)
		}
		states[id] = state
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate orders for 1c action: %w", err)
	}

	missing := make([]int, 0)
	missingReference := make([]int, 0)
	for _, orderID := range orderIDs {
		state, exists := states[orderID]
		if !exists {
			missing = append(missing, orderID)
			continue
		}

		if state.orderRefID == nil {
			missingReference = append(missingReference, orderID)
		}
	}

	if len(missing) > 0 {
		return webapp.NotFound(
			"orders not found",
			map[string]any{"order_ids": missing},
		)
	}
	if len(missingReference) == 0 {
		return nil
	}

	return webapp.BadRequest(
		"orders have no 1c reference",
		map[string]any{"order_ids": missingReference},
	)
}
