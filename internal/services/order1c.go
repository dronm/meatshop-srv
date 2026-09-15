package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/integration1cworker"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	"github.com/jackc/pgx/v5"
)

const order1CCorrelationPrefix = "order:"

func (s *OrderService) Create1C(ctx context.Context, id int) (models.Integration1CJobResponse, error) {
	if err := s.requireSession(); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	if id <= 0 {
		return models.Integration1CJobResponse{}, webapp.BadRequest("order id is required", nil)
	}

	var response models.Integration1CJobResponse
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var version int64
		if err := tx.QueryRow(ctx, `SELECT version FROM public.orders WHERE id = $1 FOR UPDATE`, id).Scan(&version); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("order not found", map[string]any{"id": id})
			}
			return err
		}
		var err error
		response, err = enqueueOrder1CSync(ctx, tx, id, version)
		return err
	}); err != nil {
		return models.Integration1CJobResponse{}, err
	}
	return response, nil
}

func enqueueOrder1CSync(ctx context.Context, tx ds.Querier, orderID int, orderVersion int64) (models.Integration1CJobResponse, error) {
	if orderID <= 0 || orderVersion <= 0 {
		return models.Integration1CJobResponse{}, webapp.BadRequest("order id and version are required for 1c sync", nil)
	}

	var customerRef *string
	if err := tx.QueryRow(ctx, `
		SELECT customer.ref_1c->>'id'
		FROM public.orders orders
		JOIN public.customers customer ON customer.id = orders.customer_id
		WHERE orders.id = $1
	`, orderID).Scan(&customerRef); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return models.Integration1CJobResponse{}, webapp.NotFound("order not found", map[string]any{"id": orderID})
		}
		return models.Integration1CJobResponse{}, err
	}
	if customerRef == nil || strings.TrimSpace(*customerRef) == "" {
		return models.Integration1CJobResponse{}, webapp.BadRequest("order customer has no 1c reference", map[string]any{"order_id": orderID})
	}

	products, err := order1CProducts(ctx, tx, orderID)
	if err != nil {
		return models.Integration1CJobResponse{}, err
	}
	params, err := json.Marshal(integration.CreateOrderParams{OrderID: orderID, OrderVersion: orderVersion, CustomerID: strings.TrimSpace(*customerRef), Products: products})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_order params: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{"entity": "order", "order_id": orderID, "order_version": orderVersion})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_order metadata: %w", err)
	}
	correlationID := fmt.Sprintf("%s%d:v%d", order1CCorrelationPrefix, orderID, orderVersion)

	var response models.Integration1CJobResponse
	if err := tx.QueryRow(ctx, `
		INSERT INTO integration_1c.jobs (command, params, correlation_id, metadata)
		VALUES ($1, $2::jsonb, $3, $4::jsonb)
		ON CONFLICT DO NOTHING
		RETURNING id, status
	`, integration.CommandCreateOrder, params, correlationID, metadata).Scan(&response.JobID, &response.Status); err != nil {
		if !errors.Is(err, ds.ErrNoRows) {
			return models.Integration1CJobResponse{}, fmt.Errorf("enqueue order %d version %d for 1c: %w", orderID, orderVersion, err)
		}
		if err := tx.QueryRow(ctx, `SELECT id, status FROM integration_1c.jobs WHERE command = $1 AND correlation_id = $2 ORDER BY id DESC LIMIT 1`, integration.CommandCreateOrder, correlationID).Scan(&response.JobID, &response.Status); err != nil {
			return models.Integration1CJobResponse{}, err
		}
	}
	return response, nil
}

func order1CProducts(ctx context.Context, tx ds.Querier, orderID int) ([]integration.CreateOrderProduct, error) {
	rows, err := tx.Query(ctx, `
		SELECT item.line_num, product.ref_1c->>'id', COALESCE(item.quant, item.quant_required)::double precision
		FROM public.order_items item
		JOIN public.products product ON product.id = item.product_id
		WHERE item.order_id = $1
		ORDER BY item.line_num, item.id
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order products for create_order: %w", err)
	}
	defer rows.Close()
	products := make([]integration.CreateOrderProduct, 0)
	for rows.Next() {
		var lineNum int
		var productRef *string
		var quant float64
		if err := rows.Scan(&lineNum, &productRef, &quant); err != nil {
			return nil, err
		}
		if productRef == nil || strings.TrimSpace(*productRef) == "" {
			return nil, webapp.BadRequest("order product has no 1c reference", map[string]any{"order_id": orderID, "line_num": lineNum})
		}
		products = append(products, integration.CreateOrderProduct{ID: strings.TrimSpace(*productRef), Quant: quant})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, webapp.BadRequest("order should contain at least one item", map[string]any{"order_id": orderID})
	}
	return products, nil
}

func HandleCreateOrder1CResult(ctx context.Context, tx pgx.Tx, result integration1cworker.Result) error {
	orderID, orderVersion, err := orderSyncMetadata(result.Metadata)
	if err != nil {
		if legacyOrderID, ok := orderIDFromLegacyCorrelation(result.CorrelationID); ok {
			orderID = legacyOrderID
			orderVersion = 0
		} else {
			slog.Error("invalid create_order result metadata", "jobID", result.JobID, "err", err)
			return nil
		}
	}
	var currentVersion int64
	if err := tx.QueryRow(ctx, `SELECT version FROM public.orders WHERE id = $1`, orderID).Scan(&currentVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if orderVersion < currentVersion && result.HTTPStatus != nil && *result.HTTPStatus == http.StatusNoContent {
		return nil
	}
	if result.Outcome != "http_response" || result.HTTPStatus == nil || *result.HTTPStatus < 200 || *result.HTTPStatus >= 300 {
		return nil
	}
	if *result.HTTPStatus == http.StatusNoContent {
		return nil
	}

	var response integration.CreateOrderResponse
	if err := json.Unmarshal(result.Body, &response); err != nil {
		slog.Error("decode create_order response", "orderID", orderID, "jobID", result.JobID, "err", err)
		return nil
	}
	if !response.Success || response.Payload == nil || strings.TrimSpace(response.Payload.ID) == "" {
		return nil
	}
	refJSON, err := json.Marshal(
		models.Ref1c{
			ID:    strings.TrimSpace(response.Payload.ID),
			Descr: strings.TrimSpace(response.Payload.Descr),
		},
	)
	if err != nil {
		return err
	}

	number1C := strings.TrimSpace(response.Payload.Number1C)

	if _, err := tx.Exec(
		ctx,
		`UPDATE public.orders
		SET
			ref_1c = $2::jsonb,
			number_1c = NULLIF($3, '')
		WHERE id = $1`,
		orderID,
		refJSON,
		number1C,
	); err != nil {
		return fmt.Errorf("apply create_order result to order %d: %w", orderID, err)
	}
	return nil
}

func OrderSyncJobVersion(metadata json.RawMessage) (int, int64, error) {
	return orderSyncMetadata(metadata)
}

func orderIDFrom1CResult(result integration1cworker.Result) (int, error) {
	orderID, _, err := orderSyncMetadata(result.Metadata)
	if err == nil {
		return orderID, nil
	}
	if legacyOrderID, ok := orderIDFromLegacyCorrelation(result.CorrelationID); ok {
		return legacyOrderID, nil
	}
	return 0, err
}

func orderSyncMetadata(metadata json.RawMessage) (int, int64, error) {
	var value struct {
		OrderID      int   `json:"order_id"`
		OrderVersion int64 `json:"order_version"`
	}
	if err := json.Unmarshal(metadata, &value); err != nil {
		return 0, 0, fmt.Errorf("decode order sync metadata: %w", err)
	}
	if value.OrderID <= 0 {
		return 0, 0, fmt.Errorf("order_id is missing")
	}

	// order_version was introduced with version-aware idempotent synchronization.
	// A zero version is intentionally accepted so results from jobs queued before
	// this change can still be consumed.
	return value.OrderID, value.OrderVersion, nil
}

func orderIDFromLegacyCorrelation(correlationID *string) (int, bool) {
	if correlationID == nil || !strings.HasPrefix(*correlationID, order1CCorrelationPrefix) {
		return 0, false
	}
	value := strings.TrimPrefix(*correlationID, order1CCorrelationPrefix)
	if index := strings.IndexByte(value, ':'); index >= 0 {
		value = value[:index]
	}
	orderID, err := strconv.Atoi(value)
	return orderID, err == nil && orderID > 0
}
