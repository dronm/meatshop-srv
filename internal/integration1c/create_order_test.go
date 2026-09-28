package integration1c

import (
	"encoding/json"
	"testing"
)

func TestCreateOrderResponse(t *testing.T) {
	var response CreateOrderResponse
	if err := json.Unmarshal([]byte(`{
		"success": true,
		"payload": {
			"id": "document-uuid",
			"descr": "Order display description",
			"number_1c": "000001",
			"items": [
				{"order_item_id": 31, "price": "120.000000", "amount": "240.00", "vat_percent": "20.00", "vat_amount": "40.00", "use_marking": true}
			]
		}
	}`), &response); err != nil {
		t.Fatalf("Unmarshal(): %v", err)
	}
	if !response.Success || response.Payload == nil || response.Payload.ID != "document-uuid" {
		t.Fatalf("response = %#v", response)
	}
	if len(response.Payload.Items) != 1 || response.Payload.Items[0].OrderItemID != 31 || response.Payload.Items[0].VatAmount != "40.00" {
		t.Fatalf("items = %#v", response.Payload.Items)
	}
	if response.Payload.Items[0].UseMarking == nil || !*response.Payload.Items[0].UseMarking {
		t.Fatalf("use_marking = %v, want true", response.Payload.Items[0].UseMarking)
	}
}

func TestCreateOrderItemUseMarkingJSON(t *testing.T) {
	for _, tt := range []struct {
		name, payload string
		wantPresent   bool
		wantValue     bool
	}{
		{name: "true", payload: `{"use_marking":true}`, wantPresent: true, wantValue: true},
		{name: "false", payload: `{"use_marking":false}`, wantPresent: true},
		{name: "missing", payload: `{}`},
		{name: "null", payload: `{"use_marking":null}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var item CreateOrderItemResult
			if err := json.Unmarshal([]byte(tt.payload), &item); err != nil {
				t.Fatal(err)
			}
			if (item.UseMarking != nil) != tt.wantPresent {
				t.Fatalf("use_marking present = %t, want %t", item.UseMarking != nil, tt.wantPresent)
			}
			if tt.wantPresent && *item.UseMarking != tt.wantValue {
				t.Fatalf("use_marking = %t, want %t", *item.UseMarking, tt.wantValue)
			}
		})
	}
}

func TestCreateOrderRequestItemIdentity(t *testing.T) {
	request := CreateOrderParams{
		OrderID:      42,
		OrderVersion: 3,
		Products:     []CreateOrderProduct{{ID: "1c-product", Quant: 2, OrderItemID: 31}},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Products []struct {
			OrderItemID int `json:"order_item_id"`
		} `json:"products"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Products) != 1 || wire.Products[0].OrderItemID != 31 {
		t.Fatalf("request = %s", encoded)
	}
}
