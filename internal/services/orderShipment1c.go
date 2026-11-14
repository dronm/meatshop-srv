package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/integration1cworker"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
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
	// The caller holds every selected parent row FOR UPDATE, serializing this
	// readiness check and snapshot with corrections and their queue insertions.
	versions, err := shipmentInvoiceVersions(ctx, tx, orderIDs)
	if err != nil {
		return models.Integration1CJobResponse{}, err
	}
	params, err := json.Marshal(integration.CreateShipmentsParams{
		OrderIDs:      append([]int(nil), orderIDs...),
		OrderVersions: versions,
	})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_shipments params: %w", err)
	}
	metadata, err := json.Marshal(integration.CreateShipmentsParams{
		OrderIDs:      append([]int(nil), orderIDs...),
		OrderVersions: versions,
	})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_shipments metadata: %w", err)
	}
	correlationID := createShipments1CCorrelationID(orderIDs, versions)

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

func createShipments1CCorrelationID(orderIDs []int, versions []integration.ShipmentOrderVersion) string {
	canonicalIDs := append([]int(nil), orderIDs...)
	sort.Ints(canonicalIDs)
	canonicalVersions := append([]integration.ShipmentOrderVersion(nil), versions...)
	sort.Slice(canonicalVersions, func(i, j int) bool { return canonicalVersions[i].OrderID < canonicalVersions[j].OrderID })
	value, _ := json.Marshal(integration.CreateShipmentsParams{OrderIDs: canonicalIDs, OrderVersions: canonicalVersions})
	digest := sha256.Sum256(value)
	return fmt.Sprintf("%s%x", createShipments1CCorrelationPrefix, digest)
}

type shipmentInvoiceSync struct {
	jobID      *int64
	command    *string
	status     *string
	params     json.RawMessage
	metadata   json.RawMessage
	outcome    *string
	httpStatus *int
	body       []byte
	consumed   bool
	active     bool
}

func shipmentInvoiceVersions(ctx context.Context, tx ds.Querier, orderIDs []int) ([]integration.ShipmentOrderVersion, error) {
	versions := make([]integration.ShipmentOrderVersion, 0, len(orderIDs))
	for _, id := range orderIDs {
		var version int64
		var invoiceRef string
		var sync shipmentInvoiceSync
		if err := tx.QueryRow(ctx, `
			SELECT o.version, COALESCE(o.ref_1c->>'id', ''), latest.id, latest.command,
				latest.status, latest.params, latest.metadata, r.outcome, r.http_status,
				r.body, COALESCE(r.consumed_at IS NOT NULL, FALSE),
				EXISTS (SELECT 1 FROM integration_1c.jobs AS pending
					WHERE pending.status IN ('queued', 'processing') AND (
						(pending.command = 'create_order' AND pending.params->>'order_id' = $2)
						OR (pending.command = 'create_orders' AND pending.params->'orders' @> $3::jsonb)
					))
			FROM public.orders AS o
			LEFT JOIN LATERAL (
				SELECT j.* FROM integration_1c.jobs AS j
				WHERE (j.command = 'create_order' AND j.params->>'order_id' = $2)
					OR (j.command = 'create_orders' AND j.params->'orders' @> $3::jsonb)
				ORDER BY j.id DESC LIMIT 1
			) AS latest ON TRUE
			LEFT JOIN integration_1c.results AS r ON r.job_id = latest.id
			WHERE o.id = $1
		`, id, fmt.Sprint(id), fmt.Sprintf(`[{"order_id":%d}]`, id)).Scan(
			&version, &invoiceRef, &sync.jobID, &sync.command, &sync.status, &sync.params, &sync.metadata,
			&sync.outcome, &sync.httpStatus, &sync.body, &sync.consumed, &sync.active,
		); err != nil {
			return nil, fmt.Errorf("load invoice synchronization before shipment: %w", err)
		}
		// A previously linked/imported invoice without queue history remains usable.
		if sync.jobID == nil && !sync.active {
			continue
		}
		current, err := order1CParams(ctx, tx, id, version)
		if err != nil {
			return nil, err
		}
		if !shipmentInvoiceSynchronized(current, invoiceRef, sync) {
			return nil, webapp.Conflict("Дождитесь успешного обновления счета 1С перед созданием отгрузки.",
				map[string]any{"order_id": id, "job_id": sync.jobID})
		}
		versions = append(versions, integration.ShipmentOrderVersion{OrderID: id, OrderVersion: version})
	}
	return versions, nil
}

func shipmentInvoiceSynchronized(current integration.CreateOrderParams, invoiceRef string, sync shipmentInvoiceSync) bool {
	if sync.active || sync.status == nil || *sync.status != "completed" || !sync.consumed ||
		sync.command == nil || sync.outcome == nil || *sync.outcome != "http_response" ||
		sync.httpStatus == nil || *sync.httpStatus < http.StatusOK || *sync.httpStatus >= http.StatusMultipleChoices ||
		*sync.httpStatus == http.StatusNoContent {
		return false
	}
	var snapshot integration.CreateOrderParams
	var result integration.CreateOrderResult
	switch *sync.command {
	case integration.CommandCreateOrder:
		var response integration.CreateOrderResponse
		if json.Unmarshal(sync.params, &snapshot) != nil || json.Unmarshal(sync.body, &response) != nil ||
			!response.Success || response.Payload == nil {
			return false
		}
		result = *response.Payload
	case integration.CommandCreateOrders:
		params, err := CreateOrdersJobParams(sync.params)
		var response integration.CreateOrdersResponse
		if err != nil || json.Unmarshal(sync.body, &response) != nil {
			return false
		}
		for index := range response.Payload {
			normalizeLegacyOrder1CMarking(sync.metadata, response.Payload[index].Items)
		}
		if validateCreateOrdersResult(params, response) != nil {
			return false
		}
		for _, order := range params.Orders {
			if order.OrderID == current.OrderID {
				snapshot = order
			}
		}
		for _, order := range response.Payload {
			if order.OrderID == current.OrderID {
				result = order.CreateOrderResult
			}
		}
	default:
		return false
	}
	snapshot.CustomerName, snapshot.CustomerPhone = trimOptionalString(snapshot.CustomerName), trimOptionalString(snapshot.CustomerPhone)
	if !reflect.DeepEqual(current, snapshot) || strings.TrimSpace(result.ID) == "" || strings.TrimSpace(result.ID) != strings.TrimSpace(invoiceRef) {
		return false
	}
	if result.Items == nil {
		// A versioned snapshot with local line IDs requires returned calculations;
		// the result consumer rejects a header-only response for such a job too.
		return false
	}
	normalizeLegacyOrder1CMarking(sync.metadata, result.Items)
	if _, err := validateOrder1CItemResults(snapshot, result.Items); err != nil {
		return false
	}
	return true
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
			SET shipment_ref_1c = $2::jsonb,
				shipment_number_1c = COALESCE(
					NULLIF($3::text, ''),
					CASE WHEN (shipment_ref_1c->>'id') = ($2::jsonb->>'id')
						THEN shipment_number_1c
					END
				)
			WHERE id = $1
		`, shipment.OrderID, refJSON, strings.TrimSpace(shipment.Number1C))
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
