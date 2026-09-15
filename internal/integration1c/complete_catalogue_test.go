package integration1c

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompleteProducts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/execute" {
			t.Fatalf("path = %s, want /execute", r.URL.Path)
		}

		var req commandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Command != commandCompleteCatalogue {
			t.Fatalf("command = %q, want %q", req.Command, commandCompleteCatalogue)
		}

		params, ok := req.Params.(map[string]any)
		if !ok {
			t.Fatalf("params type = %T", req.Params)
		}
		if params["catalogue_type"] != string(CatalogueNomenclature) {
			t.Fatalf("catalogue_type = %v", params["catalogue_type"])
		}
		if params["name"] != "мяс" {
			t.Fatalf("name = %v", params["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"payload": [
				{"id":"aaaaaaaa-bbbb-cccc","name":"Мясо замороженное"},
				{"id":"aaaaaaaa-bbbb-dddd","name":"Мясо охлажденное"}
			]
		}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	items, err := client.CompleteProducts(context.Background(), "мяс")
	if err != nil {
		t.Fatalf("CompleteProducts() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != "aaaaaaaa-bbbb-cccc" || items[0].Name != "Мясо замороженное" {
		t.Fatalf("items[0] = %#v", items[0])
	}
}

func TestCompleteCounterparties(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"payload": [
				{"id":"aaaaaaaa-bbbb-cccc","name":"ООО \"Ромашка\"","inn":"1234567890"},
				{"id":"aaaaaaaa-bbbb-dddd","name":"ИП Иванов","inn":"789012345611"}
			]
		}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	items, err := client.CompleteCounterparties(context.Background(), "Иван")
	if err != nil {
		t.Fatalf("CompleteCounterparties() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].INN != "1234567890" {
		t.Fatalf("items[0].INN = %q", items[0].INN)
	}
}

func TestCompleteProductsRetriesRetriableHTTPStatus(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNo := requests.Add(1)
		if requestNo < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"payload":[{"id":"1","name":"Мясо"}]}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 2)
	items, err := client.CompleteProducts(context.Background(), "мяс")
	if err != nil {
		t.Fatalf("CompleteProducts() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("requests = %d, want 3", got)
	}
}

func TestCompleteProductsDoesNotRetryBadRequest(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 2)
	_, err := client.CompleteProducts(context.Background(), "мяс")
	if err == nil {
		t.Fatal("CompleteProducts() error = nil")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestCompleteProductsReturnsCommandError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error":"1C command failed"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 2)
	_, err := client.CompleteProducts(context.Background(), "мяс")
	if err == nil {
		t.Fatal("CompleteProducts() error = nil")
	}

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("error type = %T, want *CommandError", err)
	}
	if commandErr.Message != "1C command failed" {
		t.Fatalf("command error message = %q", commandErr.Message)
	}
}

func newTestClient(t *testing.T, serverURL string, maxRetries int) *Client {
	t.Helper()

	client, err := NewClient(Config{
		URL:        serverURL,
		Timeout:    time.Second,
		MaxRetries: maxRetries,
		RetryDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return client
}
