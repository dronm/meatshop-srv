package services

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCreateShipments1CCorrelationIDUsesOrderSet(t *testing.T) {
	first := createShipments1CCorrelationID([]int{101, 102})
	reordered := createShipments1CCorrelationID([]int{102, 101})
	different := createShipments1CCorrelationID([]int{101, 103})

	if first != reordered {
		t.Fatalf("correlation differs for the same order set: %q != %q", first, reordered)
	}
	if first == different {
		t.Fatalf("correlation is equal for different order sets: %q", first)
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
