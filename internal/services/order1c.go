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

const (
	order1CCorrelationPrefix       = "order:"
	createOrderResultSchemaVersion = 2
)

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
	// These values describe the last completed 1C calculation. A new
	// synchronization invalidates them until the matching response arrives.
	if _, err := tx.Exec(ctx, `
		UPDATE public.order_items
		SET price = NULL, amount = NULL, vat_percent = NULL, vat_amount = NULL,
			use_marking = FALSE
		WHERE order_id = $1
			AND (price IS NOT NULL OR amount IS NOT NULL OR vat_percent IS NOT NULL
				OR vat_amount IS NOT NULL OR use_marking IS DISTINCT FROM FALSE)
	`, orderID); err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("clear order %d prices before 1c sync: %w", orderID, err)
	}
	params, err := json.Marshal(integration.CreateOrderParams{OrderID: orderID, OrderVersion: orderVersion, CustomerID: strings.TrimSpace(*customerRef), Products: products})
	if err != nil {
		return models.Integration1CJobResponse{}, fmt.Errorf("marshal create_order params: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"entity": "order", "order_id": orderID, "order_version": orderVersion,
		"result_schema_version": createOrderResultSchemaVersion,
	})
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
		SELECT item.id, item.line_num, product.ref_1c->>'id', COALESCE(item.quant, item.quant_required)::double precision
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
		var itemID, lineNum int
		var productRef *string
		var quant float64
		if err := rows.Scan(&itemID, &lineNum, &productRef, &quant); err != nil {
			return nil, err
		}
		if productRef == nil || strings.TrimSpace(*productRef) == "" {
			return nil, webapp.BadRequest("order product has no 1c reference", map[string]any{"order_id": orderID, "line_num": lineNum})
		}
		products = append(products, integration.CreateOrderProduct{ID: strings.TrimSpace(*productRef), Quant: quant, OrderItemID: itemID})
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
	if err := tx.QueryRow(ctx, `SELECT version FROM public.orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&currentVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	// The worker checks before calling 1C, but an Order can be edited while
	// that HTTP request is in flight. The row lock closes that race here.
	if orderVersion > 0 && orderVersion != currentVersion {
		return nil
	}
	// Pre-version jobs can be applied only to an Order that has never been
	// edited. Their original item identities are unknown.
	if orderVersion == 0 && currentVersion != 1 {
		return nil
	}
	if orderVersion > 0 {
		correlationID := fmt.Sprintf("%s%d:v%d", order1CCorrelationPrefix, orderID, orderVersion)
		var latestJobID int64
		if err := tx.QueryRow(ctx, `
			SELECT id FROM integration_1c.jobs
			WHERE command = $1 AND correlation_id = $2
			ORDER BY id DESC LIMIT 1
		`, integration.CommandCreateOrder, correlationID).Scan(&latestJobID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				slog.Error("create_order result has no matching versioned job", "orderID", orderID, "jobID", result.JobID)
				return nil
			}
			return err
		}
		if result.JobID != latestJobID {
			return nil
		}
	} else {
		// A versioned v1 job supersedes an old, unversioned result even when
		// the Order itself is still at version 1.
		var hasVersionedJob bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM integration_1c.jobs
				WHERE command = $1 AND correlation_id = $2
			)
		`, integration.CommandCreateOrder, fmt.Sprintf("%s%d:v1", order1CCorrelationPrefix, orderID)).Scan(&hasVersionedJob); err != nil {
			return err
		}
		if hasVersionedJob {
			return nil
		}
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

	var items []integration.CreateOrderItemResult
	if orderVersion == 0 {
		if response.Payload.Items != nil {
			slog.Error("create_order legacy result cannot update items", "orderID", orderID, "jobID", result.JobID)
			return nil
		}
	} else {
		var params integration.CreateOrderParams
		if err := json.Unmarshal(result.Params, &params); err != nil {
			slog.Error("decode create_order params for item result", "orderID", orderID, "jobID", result.JobID, "err", err)
			return nil
		}
		if params.OrderID != orderID || params.OrderVersion != orderVersion {
			slog.Error("create_order result metadata does not match params", "orderID", orderID, "jobID", result.JobID)
			return nil
		}
		if response.Payload.Items == nil {
			// Old queued jobs did not include local item IDs, so their
			// header-only responses cannot update monetary values.
			for _, product := range params.Products {
				if product.OrderItemID > 0 {
					slog.Error("create_order result omits prices for a new job", "orderID", orderID, "jobID", result.JobID)
					return nil
				}
			}
		} else {
			// Older queued jobs lacked marking; use the database default.
			normalizeLegacyOrder1CMarking(result.Metadata, response.Payload.Items)
			items, err = validateOrder1CItemResults(params, response.Payload.Items)
			if err != nil {
				slog.Error("invalid create_order item result", "orderID", orderID, "jobID", result.JobID, "err", err)
				return nil
			}
			matches, err := order1CStoredItemsMatch(ctx, tx, orderID, params)
			if err != nil {
				return err
			}
			if !matches {
				slog.Error("create_order result items no longer match Order", "orderID", orderID, "jobID", result.JobID)
				return nil
			}
		}
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
	for _, item := range items {
		tag, err := tx.Exec(ctx, `
			UPDATE public.order_items
			SET price = $3::text::numeric,
				amount = $4::text::numeric,
				vat_percent = $5::text::numeric,
				vat_amount = $6::text::numeric,
				use_marking = $7
			WHERE id = $1 AND order_id = $2
		`, item.OrderItemID, orderID, item.Price, item.Amount, item.VatPercent, item.VatAmount, *item.UseMarking)
		if err != nil {
			return fmt.Errorf("apply create_order result to order item %d: %w", item.OrderItemID, err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("create_order item %d disappeared while applying order %d", item.OrderItemID, orderID)
		}
	}
	return nil
}

// validateOrder1CItemResults checks the complete immutable request snapshot
// before the caller writes any returned values. Product references can repeat,
// so only the local Order item ID is used to associate the response rows.
func validateOrder1CItemResults(
	params integration.CreateOrderParams,
	items []integration.CreateOrderItemResult,
) ([]integration.CreateOrderItemResult, error) {
	if len(params.Products) == 0 || len(items) != len(params.Products) {
		return nil, fmt.Errorf("expected %d item results, got %d", len(params.Products), len(items))
	}
	expected := make(map[int]struct{}, len(params.Products))
	for _, product := range params.Products {
		if product.OrderItemID <= 0 {
			return nil, fmt.Errorf("request contains an invalid order_item_id %d", product.OrderItemID)
		}
		if _, duplicate := expected[product.OrderItemID]; duplicate {
			return nil, fmt.Errorf("request repeats order_item_id %d", product.OrderItemID)
		}
		expected[product.OrderItemID] = struct{}{}
	}
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if _, ok := expected[item.OrderItemID]; !ok {
			return nil, fmt.Errorf("unexpected order_item_id %d", item.OrderItemID)
		}
		if _, duplicate := seen[item.OrderItemID]; duplicate {
			return nil, fmt.Errorf("duplicate result for order_item_id %d", item.OrderItemID)
		}
		seen[item.OrderItemID] = struct{}{}
		if item.UseMarking == nil {
			return nil, fmt.Errorf("order_item_id %d use_marking must be a boolean", item.OrderItemID)
		}
		for _, field := range []struct {
			name          string
			value         string
			integerDigits int
			scale         int
		}{
			{"price", item.Price, 13, 6},
			{"amount", item.Amount, 13, 2},
			{"vat_percent", item.VatPercent, 3, 2},
			{"vat_amount", item.VatAmount, 13, 2},
		} {
			if err := validateOrder1CDecimal(field.value, field.integerDigits, field.scale); err != nil {
				return nil, fmt.Errorf("order_item_id %d %s: %w", item.OrderItemID, field.name, err)
			}
		}
		percentParts := strings.SplitN(item.VatPercent, ".", 2)
		percentInteger := strings.TrimLeft(percentParts[0], "0")
		if percentInteger == "" {
			percentInteger = "0"
		}
		percent, _ := strconv.Atoi(percentInteger) // At most three digits after validation.
		if percent > 100 || (percent == 100 && len(percentParts) == 2 && strings.Trim(percentParts[1], "0") != "") {
			return nil, fmt.Errorf("order_item_id %d vat_percent exceeds 100", item.OrderItemID)
		}
	}
	return items, nil
}

func validateOrder1CDecimal(value string, integerDigits, scale int) error {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return fmt.Errorf("must be a nonnegative decimal string")
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("must be a nonnegative decimal string")
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return fmt.Errorf("must be a nonnegative decimal string")
			}
		}
	}
	whole := strings.TrimLeft(parts[0], "0")
	if len(whole) > integerDigits {
		return fmt.Errorf("exceeds %d integer digits", integerDigits)
	}
	if len(parts) == 2 && len(parts[1]) > scale {
		return fmt.Errorf("exceeds %d fractional digits", scale)
	}
	return nil
}

func order1CStoredItemsMatch(
	ctx context.Context,
	tx pgx.Tx,
	orderID int,
	params integration.CreateOrderParams,
) (bool, error) {
	var customerRef *string
	if err := tx.QueryRow(ctx, `
		SELECT customer.ref_1c->>'id'
		FROM public.orders orders
		JOIN public.customers customer ON customer.id = orders.customer_id
		WHERE orders.id = $1
		FOR SHARE OF customer
	`, orderID).Scan(&customerRef); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if customerRef == nil || strings.TrimSpace(*customerRef) != params.CustomerID {
		return false, nil
	}
	expected := make(map[int]integration.CreateOrderProduct, len(params.Products))
	for _, product := range params.Products {
		expected[product.OrderItemID] = product
	}
	rows, err := tx.Query(ctx, `
		SELECT item.id, product.ref_1c->>'id', COALESCE(item.quant, item.quant_required)::double precision
		FROM public.order_items item
		JOIN public.products product ON product.id = item.product_id
		WHERE item.order_id = $1
		FOR UPDATE OF item
		FOR SHARE OF product
	`, orderID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	count := 0
	matches := true
	for rows.Next() {
		var itemID int
		var productRef *string
		var quant float64
		if err := rows.Scan(&itemID, &productRef, &quant); err != nil {
			return false, err
		}
		count++
		product, ok := expected[itemID]
		if !ok || productRef == nil || strings.TrimSpace(*productRef) != product.ID || quant != product.Quant {
			matches = false
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return matches && count == len(params.Products), nil
}

func OrderSyncJobVersion(metadata json.RawMessage) (int, int64, error) {
	return orderSyncMetadata(metadata)
}

func order1CResultSchemaVersion(metadata json.RawMessage) int {
	var value struct {
		Version int `json:"result_schema_version"`
	}
	if err := json.Unmarshal(metadata, &value); err != nil {
		return 0
	}
	return value.Version
}

func normalizeLegacyOrder1CMarking(metadata json.RawMessage, items []integration.CreateOrderItemResult) {
	if order1CResultSchemaVersion(metadata) >= createOrderResultSchemaVersion {
		return
	}
	unmarked := false
	for i := range items {
		if items[i].UseMarking == nil {
			items[i].UseMarking = &unmarked
		}
	}
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
