package services

import (
	"context"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/webapp"
)

func GetOrderPrint1C(
	ctx context.Context,
	db ds.Provider,
	client *integration.Client,
	orderIDs []int,
) (integration.BinaryResponse, error) {
	if err := requireOrder1CDependencies(db, client); err != nil {
		return integration.BinaryResponse{}, err
	}
	if err := validateOrderIDBatch(orderIDs); err != nil {
		return integration.BinaryResponse{}, err
	}
	if err := validateOrdersFor1CAction(
		ctx,
		db,
		orderIDs,
	); err != nil {
		return integration.BinaryResponse{}, err
	}

	result, err := client.PrintOrder(ctx, orderIDs)
	if err != nil {
		return integration.BinaryResponse{}, webapp.Internal(
			"1c order print failed",
			map[string]any{"error": err.Error()},
		)
	}

	return result, nil
}

func GetShipmentPrint1C(
	ctx context.Context,
	db ds.Provider,
	client *integration.Client,
	orderIDs []int,
) (integration.BinaryResponse, error) {
	if err := requireOrder1CDependencies(db, client); err != nil {
		return integration.BinaryResponse{}, err
	}
	if err := validateOrderIDBatch(orderIDs); err != nil {
		return integration.BinaryResponse{}, err
	}
	// The print_shipment wire contract resolves shipments from local order IDs.
	// A shipment_ref_1c value is therefore useful metadata, but not a request
	// precondition (the current mock create_shipments response has no references).
	if err := validateOrdersFor1CAction(
		ctx,
		db,
		orderIDs,
	); err != nil {
		return integration.BinaryResponse{}, err
	}

	result, err := client.PrintShipment(ctx, orderIDs)
	if err != nil {
		return integration.BinaryResponse{}, webapp.Internal(
			"1c shipment print failed",
			map[string]any{"error": err.Error()},
		)
	}

	return result, nil
}

func requireOrder1CDependencies(db ds.Provider, client *integration.Client) error {
	if db == nil {
		return webapp.Internal("database is not initialized", nil)
	}
	if client == nil {
		return webapp.Internal("1c integration is not configured", nil)
	}
	return nil
}
