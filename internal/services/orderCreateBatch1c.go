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
	"strconv"
	"strings"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/integration1cworker"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	"github.com/jackc/pgx/v5"
)

// CreateOrders1C snapshots the selected orders in one durable batch job.
func (s *OrderService) CreateOrders1C(ctx context.Context, input models.OrderIDsRequest) (models.Integration1CJobResponse, error) {
	if err := s.requireSession(); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if err := validateOrderIDBatch(input.OrderIDs); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if len(input.OrderIDs) > 1000 {
		return models.Integration1CJobResponse{}, webapp.BadRequest("too many order ids", nil)
	}

	orderIDs := append([]int(nil), input.OrderIDs...)
	sort.Ints(orderIDs)
	var response models.Integration1CJobResponse
	err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		// Lock every parent before reading item snapshots or checking active jobs.
		rows, err := tx.Query(ctx, `SELECT id, version FROM public.orders WHERE id = ANY($1) ORDER BY id FOR UPDATE`, orderIDs)
		if err != nil {
			return err
		}
		versions := make(map[int]int64, len(orderIDs))
		for rows.Next() {
			var id int
			var version int64
			if err := rows.Scan(&id, &version); err != nil {
				rows.Close()
				return err
			}
			versions[id] = version
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		params := integration.CreateOrdersParams{Orders: make([]integration.CreateOrderParams, 0, len(orderIDs))}
		for _, id := range orderIDs {
			version, exists := versions[id]
			if !exists {
				return webapp.NotFound("order not found", map[string]any{"id": id})
			}
			order, err := order1CParams(ctx, tx, id, version)
			if err != nil {
				return err
			}
			params.Orders = append(params.Orders, order)
		}
		rawParams, err := json.Marshal(params)
		if err != nil {
			return err
		}
		correlationID := createOrders1CCorrelationID(params)
		err = tx.QueryRow(ctx, `SELECT id, status FROM integration_1c.jobs
			WHERE command = $1 AND correlation_id = $2 AND status IN ('queued', 'processing')
			ORDER BY id DESC LIMIT 1`, integration.CommandCreateOrders, correlationID).Scan(&response.JobID, &response.Status)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ds.ErrNoRows) {
			return err
		}
		if err := ensureNoActiveOrderCreation(ctx, tx, orderIDs, true); err != nil {
			return err
		}
		for _, id := range orderIDs {
			if err := clearOrder1CPrices(ctx, tx, id); err != nil {
				return err
			}
		}
		metadata, err := json.Marshal(struct {
			OrderIDs            []int `json:"order_ids"`
			ResultSchemaVersion int   `json:"result_schema_version"`
		}{orderIDs, createOrderResultSchemaVersion})
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO integration_1c.jobs (command, params, correlation_id, metadata)
			VALUES ($1, $2::jsonb, $3, $4::jsonb) RETURNING id, status`,
			integration.CommandCreateOrders, rawParams, correlationID, metadata).Scan(&response.JobID, &response.Status)
	})
	if err != nil {
		return models.Integration1CJobResponse{}, err
	}
	return response, nil
}

func createOrders1CCorrelationID(params integration.CreateOrdersParams) string {
	orders := append([]integration.CreateOrderParams(nil), params.Orders...)
	sort.Slice(orders, func(i, j int) bool { return orders[i].OrderID < orders[j].OrderID })
	data, _ := json.Marshal(integration.CreateOrdersParams{Orders: orders})
	return fmt.Sprintf("orders:%x", sha256.Sum256(data))
}

// Parent row locks serialize this check with both single and batch enqueueing.
func ensureNoActiveOrderCreation(ctx context.Context, tx ds.Querier, orderIDs []int, includeSingle bool) error {
	ids := make([]string, len(orderIDs))
	for i, id := range orderIDs {
		ids[i] = strconv.Itoa(id)
	}
	var jobID int64
	err := tx.QueryRow(ctx, `SELECT id FROM integration_1c.jobs
		WHERE status IN ('queued', 'processing') AND (
			($2::boolean AND command = 'create_order' AND params->>'order_id' = ANY($1::text[]))
			OR (command = 'create_orders' AND EXISTS (
				SELECT 1 FROM jsonb_array_elements(params->'orders') AS requested
				WHERE requested->>'order_id' = ANY($1::text[])
			))
		) ORDER BY id LIMIT 1`, ids, includeSingle).Scan(&jobID)
	if errors.Is(err, ds.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return webapp.Conflict("selected orders already have an active 1c creation job", map[string]any{"job_id": jobID})
}

// CreateOrdersJobParams is also used by the worker's stale-snapshot guard.
func CreateOrdersJobParams(data json.RawMessage) (integration.CreateOrdersParams, error) {
	var params integration.CreateOrdersParams
	if err := json.Unmarshal(data, &params); err != nil {
		return params, err
	}
	ids := make([]int, 0, len(params.Orders))
	for _, order := range params.Orders {
		if order.OrderVersion <= 0 || strings.TrimSpace(order.CustomerID) == "" || len(order.Products) == 0 {
			return params, fmt.Errorf("invalid create_orders snapshot for order %d", order.OrderID)
		}
		ids = append(ids, order.OrderID)
	}
	return params, integration.ValidateOrderIDs(ids)
}

func validateCreateOrdersResult(params integration.CreateOrdersParams, response integration.CreateOrdersResponse) error {
	if !response.Success || len(response.Payload) != len(params.Orders) {
		return fmt.Errorf("create_orders must return exactly one successful result per order: %s", response.Error)
	}
	expected := make(map[int]integration.CreateOrderParams, len(params.Orders))
	for _, order := range params.Orders {
		expected[order.OrderID] = order
	}
	for _, result := range response.Payload {
		order, exists := expected[result.OrderID]
		if !exists || result.OrderVersion != order.OrderVersion || strings.TrimSpace(result.ID) == "" {
			return fmt.Errorf("unexpected, duplicate, or invalid order result: %d", result.OrderID)
		}
		if _, err := validateOrder1CItemResults(order, result.Items); err != nil {
			return fmt.Errorf("order %d: %w", order.OrderID, err)
		}
		delete(expected, result.OrderID)
	}
	return nil
}

func HandleCreateOrders1CResult(ctx context.Context, tx pgx.Tx, result integration1cworker.Result) error {
	if result.Outcome != "http_response" || result.HTTPStatus == nil || *result.HTTPStatus < 200 ||
		*result.HTTPStatus >= 300 || *result.HTTPStatus == http.StatusNoContent {
		return nil
	}
	params, err := CreateOrdersJobParams(result.Params)
	if err != nil {
		return err
	}
	var response integration.CreateOrdersResponse
	if err := json.Unmarshal(result.Body, &response); err != nil {
		slog.Error("decode create_orders response", "jobID", result.JobID, "err", err)
		return nil
	}
	if err := validateCreateOrdersResult(params, response); err != nil {
		slog.Error("invalid create_orders result", "jobID", result.JobID, "err", err)
		return nil
	}
	byID := make(map[int]integration.CreateOrdersResult, len(response.Payload))
	for _, order := range response.Payload {
		byID[order.OrderID] = order
	}
	// Apply in canonical order so overlapping completed batches take row locks
	// consistently. Each order retains the existing version/item checks.
	sort.Slice(params.Orders, func(i, j int) bool { return params.Orders[i].OrderID < params.Orders[j].OrderID })
	for _, order := range params.Orders {
		item := byID[order.OrderID]
		single := result
		single.Params, _ = json.Marshal(order)
		single.Metadata, _ = json.Marshal(map[string]any{
			"order_id": order.OrderID, "order_version": order.OrderVersion,
			"result_schema_version": createOrderResultSchemaVersion,
		})
		single.Body, _ = json.Marshal(integration.CreateOrderResponse{Success: true, Payload: &item.CreateOrderResult})
		if err := HandleCreateOrder1CResult(ctx, tx, single); err != nil {
			return err
		}
	}
	return nil
}
