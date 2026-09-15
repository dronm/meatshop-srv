package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/dronm/ds/v4"
)

const orderRecorderType = "Order"

func withPrimaryTransaction(
	ctx context.Context,
	db ds.Provider,
	fn func(ds.Tx) error,
) error {
	poolConn, connID, err := db.GetPrimary(ctx)
	if err != nil {
		return fmt.Errorf("get primary connection: %w", err)
	}
	defer db.Release(poolConn, connID)

	if err := ds.WithTx(ctx, poolConn.Conn(), func(ctx context.Context, tx ds.Tx) error {
		return fn(tx)
	}); err != nil {
		return err
	}

	return nil
}

func lockProductRegisterRecorders(
	ctx context.Context,
	tx ds.Querier,
	recorderType string,
	recorderIDs ...int,
) error {
	ids := append([]int(nil), recorderIDs...)
	sort.Ints(ids)

	previousID := 0
	for _, recorderID := range ids {
		if recorderID <= 0 || recorderID == previousID {
			continue
		}

		if _, err := tx.Exec(
			ctx,
			"SELECT pg_advisory_xact_lock(hashtext($1), $2)",
			recorderType,
			recorderID,
		); err != nil {
			return fmt.Errorf(
				"lock %s products register recorder %d: %w",
				recorderType,
				recorderID,
				err,
			)
		}

		previousID = recorderID
	}

	return nil
}

func removeProductRegisterActions(
	ctx context.Context,
	tx ds.Querier,
	recorderType string,
	recorderID int,
) error {
	if _, err := tx.Exec(
		ctx,
		"SELECT public.ra_products_remove_acts($1, $2)",
		recorderType,
		recorderID,
	); err != nil {
		return fmt.Errorf(
			"remove products register actions for %s %d: %w",
			recorderType,
			recorderID,
			err,
		)
	}

	return nil
}

func rebuildProductRegisterActions(
	ctx context.Context,
	tx ds.Querier,
	recorderType string,
	recorderID int,
) error {
	if recorderType != orderRecorderType {
		return fmt.Errorf("unsupported products register recorder type %q", recorderType)
	}
	if err := removeProductRegisterActions(ctx, tx, recorderType, recorderID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		SELECT public.ra_products_add_act(
			public.register_date_start(orders.for_date),
			$1,
			orders.id,
			orders.customer_id,
			orders.customer_sale_place_id,
			item.product_id,
			item.measure_unit_id,
			item.quant_required,
			item.quant
		)
		FROM public.orders AS orders
		JOIN public.order_items AS item
			ON item.order_id = orders.id
		WHERE orders.id = $2
		ORDER BY item.line_num, item.id
	`, recorderType, recorderID); err != nil {
		return fmt.Errorf(
			"write products register actions for %s %d: %w",
			recorderType,
			recorderID,
			err,
		)
	}

	return nil
}
