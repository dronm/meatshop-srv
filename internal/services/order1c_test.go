package services

import (
	"encoding/json"
	"testing"

	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/integration1cworker"
)

func TestOrderIDFrom1CResultMetadata(t *testing.T) {
	metadata, err := json.Marshal(map[string]any{"order_id": 42})
	if err != nil {
		t.Fatal(err)
	}
	orderID, err := orderIDFrom1CResult(integration1cworker.Result{Metadata: metadata})
	if err != nil {
		t.Fatalf("orderIDFrom1CResult(): %v", err)
	}
	if orderID != 42 {
		t.Fatalf("order id = %d", orderID)
	}
}

func TestOrderIDFrom1CResultCorrelationFallback(t *testing.T) {
	correlation := "order:77"
	orderID, err := orderIDFrom1CResult(integration1cworker.Result{CorrelationID: &correlation})
	if err != nil {
		t.Fatalf("orderIDFrom1CResult(): %v", err)
	}
	if orderID != 77 {
		t.Fatalf("order id = %d", orderID)
	}
}

func TestNormalizeLegacyOrder1CMarking(t *testing.T) {
	for _, tt := range []struct {
		name        string
		metadata    json.RawMessage
		wantDefault bool
	}{
		{name: "old queued job", metadata: json.RawMessage(`{"order_id":42,"order_version":1}`), wantDefault: true},
		{name: "new queued job", metadata: json.RawMessage(`{"order_id":42,"order_version":1,"result_schema_version":2}`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items := []integration.CreateOrderItemResult{{OrderItemID: 31}}
			normalizeLegacyOrder1CMarking(tt.metadata, items)
			if (items[0].UseMarking != nil) != tt.wantDefault {
				t.Fatalf("use_marking present = %t, want default %t", items[0].UseMarking != nil, tt.wantDefault)
			}
			if tt.wantDefault && *items[0].UseMarking {
				t.Fatal("legacy use_marking should default to false")
			}
		})
	}
}

func TestValidateOrder1CItemResults(t *testing.T) {
	params := integration.CreateOrderParams{
		Products: []integration.CreateOrderProduct{
			{OrderItemID: 31, ID: "same-product", Quant: 2},
			{OrderItemID: 32, ID: "same-product", Quant: 1},
		},
	}
	item := func(id int) integration.CreateOrderItemResult {
		unmarked := false
		return integration.CreateOrderItemResult{
			OrderItemID: id,
			Price:       "123.450001",
			Amount:      "246.90",
			VatPercent:  "20.00",
			VatAmount:   "41.15",
			UseMarking:  &unmarked,
		}
	}
	marked := item(31)
	marked.UseMarking = markingBool(true)

	tests := []struct {
		name       string
		items      []integration.CreateOrderItemResult
		wantErr    bool
		wantMarked bool
	}{
		{
			name:  "same product on two lines, response reordered",
			items: []integration.CreateOrderItemResult{item(32), item(31)},
		},
		{
			name:       "true marking",
			items:      []integration.CreateOrderItemResult{marked, item(32)},
			wantMarked: true,
		},
		{
			name: "missing marking",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "123.450001", Amount: "246.90", VatPercent: "20.00", VatAmount: "41.15"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "zero VAT",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "0.000000", Amount: "0.00", VatPercent: "0.00", VatAmount: "0.00"},
				item(32),
			},
		},
		{
			name:    "missing line",
			items:   []integration.CreateOrderItemResult{item(31)},
			wantErr: true,
		},
		{
			name:    "empty lines",
			items:   []integration.CreateOrderItemResult{},
			wantErr: true,
		},
		{
			name:    "extra line",
			items:   []integration.CreateOrderItemResult{item(31), item(32), item(33)},
			wantErr: true,
		},
		{
			name:    "duplicate line",
			items:   []integration.CreateOrderItemResult{item(31), item(31)},
			wantErr: true,
		},
		{
			name: "missing price",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Amount: "246.90", VatPercent: "20.00", VatAmount: "41.15"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "nonnumeric amount",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "123.45", Amount: "invalid", VatPercent: "20.00", VatAmount: "41.15"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "price scale exceeds numeric(19,6)",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "1.1234567", Amount: "2.00", VatPercent: "20.00", VatAmount: "0.33"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "price precision exceeds numeric(19,6)",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "10000000000000.000000", Amount: "2.00", VatPercent: "20.00", VatAmount: "0.33"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "amount scale exceeds numeric(15,2)",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "1.000000", Amount: "1.001", VatPercent: "20.00", VatAmount: "0.17"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "VAT amount precision exceeds numeric(15,2)",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "1.000000", Amount: "1.00", VatPercent: "20.00", VatAmount: "10000000000000.00"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "VAT percent scale exceeds numeric(5,2)",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "1.000000", Amount: "1.00", VatPercent: "20.123", VatAmount: "0.17"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "negative price",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "-1.000000", Amount: "1.00", VatPercent: "20.00", VatAmount: "0.17"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "negative VAT amount",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "1.000000", Amount: "1.00", VatPercent: "20.00", VatAmount: "-0.17"},
				item(32),
			},
			wantErr: true,
		},
		{
			name: "VAT percent over 100",
			items: []integration.CreateOrderItemResult{
				{OrderItemID: 31, Price: "1.000000", Amount: "1.00", VatPercent: "100.01", VatAmount: "0.50"},
				item(32),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "missing marking" {
				for i := range tt.items {
					if tt.items[i].UseMarking == nil {
						tt.items[i].UseMarking = markingBool(false)
					}
				}
			}
			validated, err := validateOrder1CItemResults(params, tt.items)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateOrder1CItemResults() returned %v; want error", validated)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateOrder1CItemResults(): %v", err)
			}
			if len(validated) != len(params.Products) {
				t.Fatalf("validated items = %d, want %d", len(validated), len(params.Products))
			}
			gotIDs := make(map[int]bool, len(validated))
			for _, result := range validated {
				gotIDs[result.OrderItemID] = true
				if result.UseMarking == nil {
					t.Fatalf("validated item %d has no use_marking", result.OrderItemID)
				}
				if result.OrderItemID == 31 && *result.UseMarking != tt.wantMarked {
					t.Fatalf("item 31 use_marking = %t, want %t", *result.UseMarking, tt.wantMarked)
				}
			}
			if !gotIDs[31] || !gotIDs[32] {
				t.Fatalf("validated item IDs = %v, want 31 and 32", gotIDs)
			}
		})
	}
}

func markingBool(value bool) *bool {
	return &value
}
