package services

import (
	"encoding/json"
	"testing"

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
