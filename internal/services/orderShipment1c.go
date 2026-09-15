package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/integration1cworker"
	"github.com/dronm/meatshop/internal/models"
	"github.com/jackc/pgx/v5"
)

const createShipments1CCorrelationPrefix = "shipments:"

func (s *OrderService) CreateShipments1C(
	ctx context.Context,
	input models.OrderIDsRequest,
) (models.Integration1CJobResponse, error) {
	if err := s.requireSession(); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if err := validateOrderIDBatch(input.OrderIDs); err != nil {
		return models.Integration1CJobResponse{}, err
	}

	var response models.Integration1CJobResponse
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := validateOrdersFor1CActionQuery(
			ctx,
			tx,
			input.OrderIDs,
			true,
		); err != nil {
			return err
		}

		var err error
		response, err = enqueueCreateShipments1C(ctx, tx, input.OrderIDs)
		return err
	}); err != nil {
		return models.Integration1CJobResponse{}, err
	}

	return response, nil
}

func enqueueCreateShipments1C(
	ctx context.Context,
	tx ds.Querier,
	orderIDs []int,
) (models.Integration1CJobResponse, error) {
	params, err := json.Marshal(integration.OrderIDsParams{
		OrderIDs: append([]int(nil), orderIDs...),
	})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_shipments params: %w", err)
	}
	metadata, err := json.Marshal(models.OrderIDsRequest{
		OrderIDs: append([]int(nil), orderIDs...),
	})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_shipments metadata: %w", err)
	}
	correlationID := createShipments1CCorrelationID(orderIDs)

	var response models.Integration1CJobResponse
	if err := tx.QueryRow(ctx, `
		INSERT INTO integration_1c.jobs (command, params, correlation_id, metadata)
		VALUES ($1, $2::jsonb, $3, $4::jsonb)
		ON CONFLICT DO NOTHING
		RETURNING id, status
	`, integration.CommandCreateShipments, params, correlationID, metadata).Scan(
		&response.JobID,
		&response.Status,
	); err != nil {
		if !errors.Is(err, ds.ErrNoRows) {
			return models.Integration1CJobResponse{}, fmt.Errorf("enqueue create_shipments: %w", err)
		}
		if err := tx.QueryRow(ctx, `
			SELECT id, status
			FROM integration_1c.jobs
			WHERE command = $1
				AND correlation_id = $2
				AND status IN ('queued', 'processing')
			ORDER BY id DESC
			LIMIT 1
		`, integration.CommandCreateShipments, correlationID).Scan(
			&response.JobID,
			&response.Status,
		); err != nil {
			return models.Integration1CJobResponse{}, err
		}
	}

	return response, nil
}

func createShipments1CCorrelationID(orderIDs []int) string {
	canonicalIDs := append([]int(nil), orderIDs...)
	sort.Ints(canonicalIDs)
	value, _ := json.Marshal(canonicalIDs)
	digest := sha256.Sum256(value)
	return fmt.Sprintf("%s%x", createShipments1CCorrelationPrefix, digest)
}

func HandleCreateShipments1CResult(
	ctx context.Context,
	tx pgx.Tx,
	result integration1cworker.Result,
) error {
	if result.Outcome != "http_response" ||
		result.HTTPStatus == nil ||
		*result.HTTPStatus < http.StatusOK ||
		*result.HTTPStatus >= http.StatusMultipleChoices ||
		*result.HTTPStatus == http.StatusNoContent {
		return nil
	}

	orderIDs, err := createShipmentsOrderIDs(result.Metadata)
	if err != nil {
		slog.Error("invalid create_shipments result metadata", "jobID", result.JobID, "err", err)
		return nil
	}

	var response integration.CreateShipmentsResponse
	if err := json.Unmarshal(result.Body, &response); err != nil {
		slog.Error("decode create_shipments response", "jobID", result.JobID, "err", err)
		return nil
	}
	if !response.Success {
		slog.Error(
			"create_shipments command failed",
			"jobID",
			result.JobID,
			"error",
			strings.TrimSpace(response.Error),
		)
		return nil
	}
	if len(response.Payload) == 0 {
		return nil
	}

	requested := make(map[int]struct{}, len(orderIDs))
	for _, orderID := range orderIDs {
		requested[orderID] = struct{}{}
	}
	updated := make(map[int]struct{}, len(response.Payload))
	for _, shipment := range response.Payload {
		shipment.ID = strings.TrimSpace(shipment.ID)
		shipment.Descr = strings.TrimSpace(shipment.Descr)
		if _, exists := requested[shipment.OrderID]; !exists {
			slog.Error(
				"create_shipments returned an unexpected order id",
				"jobID",
				result.JobID,
				"orderID",
				shipment.OrderID,
			)
			continue
		}
		if shipment.ID == "" {
			slog.Error(
				"create_shipments returned an empty shipment id",
				"jobID",
				result.JobID,
				"orderID",
				shipment.OrderID,
			)
			continue
		}
		if _, exists := updated[shipment.OrderID]; exists {
			slog.Error(
				"create_shipments returned a duplicate order id",
				"jobID",
				result.JobID,
				"orderID",
				shipment.OrderID,
			)
			continue
		}

		refJSON, err := json.Marshal(models.Ref1c{
			ID:    shipment.ID,
			Descr: shipment.Descr,
		})
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE public.orders
			SET shipment_ref_1c = $2::jsonb
			WHERE id = $1
		`, shipment.OrderID, refJSON)
		if err != nil {
			return fmt.Errorf(
				"apply create_shipments result to order %d: %w",
				shipment.OrderID,
				err,
			)
		}
		if tag.RowsAffected() == 0 {
			slog.Warn(
				"create_shipments result order no longer exists",
				"jobID",
				result.JobID,
				"orderID",
				shipment.OrderID,
			)
			continue
		}
		updated[shipment.OrderID] = struct{}{}
	}

	return nil
}

func createShipmentsOrderIDs(metadata json.RawMessage) ([]int, error) {
	var value models.OrderIDsRequest
	if err := json.Unmarshal(metadata, &value); err != nil {
		return nil, fmt.Errorf("decode create_shipments metadata: %w", err)
	}
	if err := integration.ValidateOrderIDs(value.OrderIDs); err != nil {
		return nil, err
	}
	return append([]int(nil), value.OrderIDs...), nil
}
