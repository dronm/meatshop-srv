package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
	integration "github.com/dronm/meatshop/internal/integration1c"
)

type correctionQuery struct {
	ds.Querier
	requested bool
	queued    bool
	metadata  []byte
}

type correctionRow func(...any) error

func (r correctionRow) Scan(dest ...any) error { return r(dest...) }

func (q *correctionQuery) QueryRow(_ context.Context, sql string, args ...any) ds.Row {
	return correctionRow(func(dest ...any) error {
		switch {
		case strings.Contains(sql, "FROM public.orders AS o"):
			*dest[0].(*bool) = q.requested
		case strings.Contains(sql, "SELECT customer.ref_1c"):
			ref := "customer-1c"
			*dest[0].(**string) = &ref
		case strings.Contains(sql, "array_agg(id)"):
			*dest[0].(*[]int64) = []int64{77}
		case strings.Contains(sql, "INSERT INTO integration_1c.jobs"):
			q.queued = true
			q.metadata = args[3].([]byte)
			*dest[0].(*int64) = 88
			*dest[1].(*string) = "queued"
		default:
			return fmt.Errorf("unexpected query: %s", sql)
		}
		return nil
	})
}

type correctionRows struct{ read bool }

func (r *correctionRows) Close() error { return nil }
func (r *correctionRows) Err() error   { return nil }
func (r *correctionRows) Next() bool {
	if r.read {
		return false
	}
	r.read = true
	return true
}
func (r *correctionRows) Scan(dest ...any) error {
	*dest[0].(*int) = 201
	*dest[1].(*int) = 1
	ref := "product-1c"
	*dest[2].(**string) = &ref
	*dest[3].(*float64) = 2
	return nil
}
func (q *correctionQuery) Query(context.Context, string, ...any) (ds.Rows, error) {
	return &correctionRows{}, nil
}
func (q *correctionQuery) Exec(context.Context, string, ...any) (ds.ExecResult, error) {
	return nil, nil
}

func TestOrderCorrectionSynchronizesOnlyAfterFirstRequest(t *testing.T) {
	for _, requested := range []bool{false, true} {
		q := &correctionQuery{requested: requested}
		if err := syncOrder1CAfterCorrection(context.Background(), q, 101, 3); err != nil {
			t.Fatal(err)
		}
		if q.queued != requested {
			t.Fatalf("requested = %t, queued = %t", requested, q.queued)
		}
		if requested {
			var metadata struct {
				Dependencies []int64 `json:"depends_on_job_ids"`
				Version      int64   `json:"order_version"`
			}
			if err := json.Unmarshal(q.metadata, &metadata); err != nil {
				t.Fatal(err)
			}
			if len(metadata.Dependencies) != 1 || metadata.Dependencies[0] != 77 || metadata.Version != 3 {
				t.Fatalf("correction did not retain its version and pending batch dependency: %s", q.metadata)
			}
		}
	}
}

func batchOrderFixture(id, itemID int) integration.CreateOrderParams {
	return integration.CreateOrderParams{
		OrderID: id, OrderVersion: 2, CustomerID: "customer-1c",
		Products: []integration.CreateOrderProduct{{ID: "product-1c", Quant: 2, OrderItemID: itemID}},
	}
}

func batchResultFixture(order integration.CreateOrderParams) integration.CreateOrdersResult {
	return integration.CreateOrdersResult{
		OrderID: order.OrderID, OrderVersion: order.OrderVersion,
		CreateOrderResult: integration.CreateOrderResult{
			ID: "invoice-1c", Number1C: "000001",
			Items: []integration.CreateOrderItemResult{{
				OrderItemID: order.Products[0].OrderItemID,
				Price:       "120.000000", Amount: "240.00", VatPercent: "20.00", VatAmount: "40.00",
				UseMarking: markingBool(true),
			}},
		},
	}
}

func TestCreateOrdersCorrelationUsesSnapshotsAndIgnoresSelectionOrder(t *testing.T) {
	a, b := batchOrderFixture(101, 201), batchOrderFixture(102, 202)
	first := createOrders1CCorrelationID(integration.CreateOrdersParams{Orders: []integration.CreateOrderParams{a, b}})
	reversed := createOrders1CCorrelationID(integration.CreateOrdersParams{Orders: []integration.CreateOrderParams{b, a}})
	if first != reversed {
		t.Fatal("reversing selection changed the correlation ID")
	}
	b.OrderVersion++
	if first == createOrders1CCorrelationID(integration.CreateOrdersParams{Orders: []integration.CreateOrderParams{a, b}}) {
		t.Fatal("a newer order version reused the old correlation ID")
	}
	b.OrderVersion--
	b.Products[0].Quant++
	if first == createOrders1CCorrelationID(integration.CreateOrdersParams{Orders: []integration.CreateOrderParams{a, b}}) {
		t.Fatal("changed item data reused the old correlation ID")
	}
}

func TestCreateOrdersRejectsInvalidSnapshots(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"orders":[]}`, `{"orders":[{"order_id":101}]}`,
		`{"orders":[{"order_id":101,"order_version":1,"customer_id":"c","products":[{"id":"p"}]},{"order_id":101,"order_version":1,"customer_id":"c","products":[{"id":"p"}]}]}`,
	} {
		if _, err := CreateOrdersJobParams(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid snapshot was accepted: %s", raw)
		}
	}
}

func TestCreateOrdersResponseValidatesCompleteBatchBeforeApplying(t *testing.T) {
	a, b := batchOrderFixture(101, 201), batchOrderFixture(102, 202)
	params := integration.CreateOrdersParams{Orders: []integration.CreateOrderParams{a, b}}
	for _, test := range []struct {
		name      string
		change    func(*integration.CreateOrdersResponse)
		wantError bool
	}{
		{"reordered results", func(r *integration.CreateOrdersResponse) { r.Payload[0], r.Payload[1] = r.Payload[1], r.Payload[0] }, false},
		{"failure", func(r *integration.CreateOrdersResponse) { r.Success = false }, true},
		{"missing order", func(r *integration.CreateOrdersResponse) { r.Payload = r.Payload[:1] }, true},
		{"duplicate order", func(r *integration.CreateOrdersResponse) { r.Payload[1] = r.Payload[0] }, true},
		{"unexpected order", func(r *integration.CreateOrdersResponse) { r.Payload[1].OrderID = 103 }, true},
		{"wrong version", func(r *integration.CreateOrdersResponse) { r.Payload[1].OrderVersion++ }, true},
		{"missing reference", func(r *integration.CreateOrdersResponse) { r.Payload[1].ID = "" }, true},
		{"wrong order item", func(r *integration.CreateOrdersResponse) { r.Payload[1].Items[0].OrderItemID = 201 }, true},
		{"invalid amount", func(r *integration.CreateOrdersResponse) { r.Payload[1].Items[0].Amount = "-1" }, true},
		{"missing marking", func(r *integration.CreateOrdersResponse) { r.Payload[1].Items[0].UseMarking = nil }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := integration.CreateOrdersResponse{Success: true, Payload: []integration.CreateOrdersResult{batchResultFixture(a), batchResultFixture(b)}}
			test.change(&response)
			if err := validateCreateOrdersResult(params, response); (err != nil) != test.wantError {
				t.Fatalf("validation error = %v, want error = %t", err, test.wantError)
			}
		})
	}
}
