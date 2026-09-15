package integration1c

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCreateShipmentsUsesExecuteAndReturnsReferences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/execute" {
			t.Fatalf("path = %s, want /execute", r.URL.Path)
		}

		var request struct {
			Command string         `json:"command"`
			Params  OrderIDsParams `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Command != CommandCreateShipments {
			t.Fatalf("command = %q, want %q", request.Command, CommandCreateShipments)
		}
		if !reflect.DeepEqual(request.Params.OrderIDs, []int{101, 102}) {
			t.Fatalf("order_ids = %v", request.Params.OrderIDs)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"payload": [
				{"order_id": 101, "id": "shipment-101", "descr": "Shipment 101"},
				{"order_id": 102, "id": "shipment-102", "descr": "Shipment 102"}
			]
		}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	result, err := client.CreateShipments(context.Background(), []int{101, 102})
	if err != nil {
		t.Fatalf("CreateShipments() error = %v", err)
	}

	want := []CreateShipmentResult{
		{OrderID: 101, ID: "shipment-101", Descr: "Shipment 101"},
		{OrderID: 102, ID: "shipment-102", Descr: "Shipment 102"},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}
}

func TestCreateShipmentsAcceptsEmptyMockPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"payload":[]}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	result, err := client.CreateShipments(context.Background(), []int{101, 102})
	if err != nil {
		t.Fatalf("CreateShipments() error = %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("result = %#v, want an empty payload", result)
	}
}
