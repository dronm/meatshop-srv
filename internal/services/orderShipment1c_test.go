package services

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	integration "github.com/dronm/meatshop/internal/integration1c"
)

func TestCreateShipments1CCorrelationIDUsesOrderSet(t *testing.T) {
	first := createShipments1CCorrelationID([]int{101, 102}, nil)
	reordered := createShipments1CCorrelationID([]int{102, 101}, nil)
	different := createShipments1CCorrelationID([]int{101, 103}, nil)

	if first != reordered {
		t.Fatalf("correlation differs for the same order set: %q != %q", first, reordered)
	}
	if first == different {
		t.Fatalf("correlation is equal for different order sets: %q", first)
	}
}

func TestCreateShipmentsCorrelationPinsAcceptedInvoiceVersions(t *testing.T) {
	first := []integration.ShipmentOrderVersion{{OrderID: 101, OrderVersion: 2}, {OrderID: 102, OrderVersion: 3}}
	reversed := []integration.ShipmentOrderVersion{first[1], first[0]}
	if createShipments1CCorrelationID([]int{101, 102}, first) != createShipments1CCorrelationID([]int{102, 101}, reversed) {
		t.Fatal("selection order changed pinned shipment identity")
	}
	changed := append([]integration.ShipmentOrderVersion(nil), first...)
	changed[0].OrderVersion++
	if createShipments1CCorrelationID([]int{101, 102}, first) == createShipments1CCorrelationID([]int{101, 102}, changed) {
		t.Fatal("a new invoice version reused an older active shipment job")
	}
}

func shipmentSyncFixture(t *testing.T, batch bool) (integration.CreateOrderParams, shipmentInvoiceSync) {
	t.Helper()
	current := batchOrderFixture(101, 201)
	name, phone := "Иван", "+79121234567"
	current.CashPayed, current.CustomerName, current.CustomerPhone = true, &name, &phone
	jobID, status, outcome, httpStatus := int64(41), "completed", "http_response", 200
	command := integration.CommandCreateOrder
	var params any = current
	var result any = integration.CreateOrderResponse{Success: true, Payload: func() *integration.CreateOrderResult {
		result := batchResultFixture(current).CreateOrderResult
		return &result
	}()}
	if batch {
		command = integration.CommandCreateOrders
		params = integration.CreateOrdersParams{Orders: []integration.CreateOrderParams{current}}
		result = integration.CreateOrdersResponse{Success: true, Payload: []integration.CreateOrdersResult{batchResultFixture(current)}}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return current, shipmentInvoiceSync{
		jobID: &jobID, command: &command, status: &status, outcome: &outcome, httpStatus: &httpStatus,
		params: paramsJSON, metadata: json.RawMessage(`{"result_schema_version":2}`), body: resultJSON, consumed: true,
	}
}

func TestShipmentRequiresSuccessfulConsumedCurrentInvoiceSynchronization(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, test := range []struct {
			name   string
			change func(*integration.CreateOrderParams, *shipmentInvoiceSync, *string)
		}{
			{"current success", func(*integration.CreateOrderParams, *shipmentInvoiceSync, *string) {}},
			{"retry queued", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				status := "queued"
				sync.status = &status
			}},
			{"processing on another worker", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				status := "processing"
				sync.status = &status
			}},
			{"older synchronization still active", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) { sync.active = true }},
			{"unconsumed result", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) { sync.consumed = false }},
			{"terminal transport failure", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				outcome := "transport_error"
				sync.outcome = &outcome
				sync.httpStatus = nil
			}},
			{"terminal HTTP failure", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				status := 500
				sync.httpStatus = &status
			}},
			{"skipped obsolete job", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				status := 204
				sync.httpStatus = &status
			}},
			{"business failure at HTTP 200", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				sync.body = []byte(`{"success":false,"error":"failed"}`)
			}},
			{"invalid response", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) { sync.body = []byte(`{`) }},
			{"new response omits item results", func(_ *integration.CreateOrderParams, sync *shipmentInvoiceSync, _ *string) {
				if *sync.command == integration.CommandCreateOrder {
					sync.body = []byte(`{"success":true,"payload":{"id":"invoice-1c"}}`)
				} else {
					sync.body = []byte(`{"success":true,"payload":[{"order_id":101,"order_version":2,"id":"invoice-1c"}]}`)
				}
			}},
			{"newer local version", func(current *integration.CreateOrderParams, _ *shipmentInvoiceSync, _ *string) {
				current.OrderVersion++
			}},
			{"changed contact", func(current *integration.CreateOrderParams, _ *shipmentInvoiceSync, _ *string) {
				phone := "+79127654321"
				current.CustomerPhone = &phone
			}},
			{"different applied invoice", func(_ *integration.CreateOrderParams, _ *shipmentInvoiceSync, invoiceRef *string) {
				*invoiceRef = "other-invoice"
			}},
		} {
			t.Run(fmt.Sprintf("batch=%t/%s", batch, test.name), func(t *testing.T) {
				current, sync := shipmentSyncFixture(t, batch)
				invoiceRef := "invoice-1c"
				test.change(&current, &sync, &invoiceRef)
				if got, want := shipmentInvoiceSynchronized(current, invoiceRef, sync), test.name == "current success"; got != want {
					t.Fatalf("invoice ready = %t, want %t", got, want)
				}
			})
		}
	}
}

func TestCreateShipmentsOrderIDs(t *testing.T) {
	metadata := json.RawMessage(`{"order_ids":[101,102]}`)
	orderIDs, err := createShipmentsOrderIDs(metadata)
	if err != nil {
		t.Fatalf("createShipmentsOrderIDs() error = %v", err)
	}
	if !reflect.DeepEqual(orderIDs, []int{101, 102}) {
		t.Fatalf("order ids = %v", orderIDs)
	}
}

func TestCreateShipmentsOrderIDsRejectsInvalidMetadata(t *testing.T) {
	for _, metadata := range []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"order_ids":[0]}`),
		json.RawMessage(`{"order_ids":[101,101]}`),
	} {
		if _, err := createShipmentsOrderIDs(metadata); err == nil {
			t.Fatalf("metadata %s was accepted", metadata)
		}
	}
}
