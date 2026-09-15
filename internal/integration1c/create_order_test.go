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
			"descr": "Order display description"
		}
	}`), &response); err != nil {
		t.Fatalf("Unmarshal(): %v", err)
	}
	if !response.Success || response.Payload == nil || response.Payload.ID != "document-uuid" {
		t.Fatalf("response = %#v", response)
	}
}
