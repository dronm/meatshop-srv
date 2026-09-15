package integration1cworker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayExecutePreservesCommandAndRawResponse(t *testing.T) {
	responseBody := []byte(`{"success":true,"payload":{"id":"doc-1","descr":"Order 1"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/execute" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var request struct {
			Command string          `json:"command"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if request.Command != "create_order" {
			t.Fatalf("command = %q", request.Command)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	client, err := NewGatewayClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewGatewayClient(): %v", err)
	}
	response, err := client.Execute(context.Background(), "create_order", json.RawMessage(`{"customer_id":"c1"}`))
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if response.StatusCode != http.StatusOK || string(response.Body) != string(responseBody) {
		t.Fatalf("unexpected response: %#v %s", response, response.Body)
	}
}
