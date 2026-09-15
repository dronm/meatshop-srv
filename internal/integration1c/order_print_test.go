package integration1c

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestOrderPrintFormUsesBinDataAndReturnsPDF(t *testing.T) {
	pdf := []byte("%PDF-1.7\nmock pdf\n%%EOF\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/bin-data" {
			t.Fatalf("path = %s, want /bin-data", r.URL.Path)
		}

		var req commandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Command != CommandOrderPrintForm {
			t.Fatalf("command = %q, want %q", req.Command, CommandOrderPrintForm)
		}
		params, ok := req.Params.(map[string]any)
		if !ok {
			t.Fatalf("params type = %T", req.Params)
		}
		if params["order_id"] != "aaaaaaaa-bbbb-cccc" {
			t.Fatalf("order_id = %v", params["order_id"])
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="order-1c.pdf"`)
		_, _ = w.Write(pdf)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/execute", 0)
	result, err := client.OrderPrintForm(context.Background(), "aaaaaaaa-bbbb-cccc")
	if err != nil {
		t.Fatalf("OrderPrintForm() error = %v", err)
	}
	if string(result.Body) != string(pdf) {
		t.Fatalf("body = %q, want %q", result.Body, pdf)
	}
	if result.ContentType != "application/pdf" {
		t.Fatalf("content type = %q", result.ContentType)
	}
	if result.FileName != "order-1c.pdf" {
		t.Fatalf("file name = %q", result.FileName)
	}
}

func TestOrderPrintFormAcceptsOctetStreamWhenBodyIsPDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("%PDF-1.4\nmock"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	result, err := client.OrderPrintForm(context.Background(), "order-ref")
	if err != nil {
		t.Fatalf("OrderPrintForm() error = %v", err)
	}
	if result.ContentType != "application/pdf" {
		t.Fatalf("content type = %q", result.ContentType)
	}
}

func TestOrderPrintFormRejectsNonPDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not a pdf"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	if _, err := client.OrderPrintForm(context.Background(), "order-ref"); err == nil {
		t.Fatal("OrderPrintForm() error = nil")
	}
}

func TestOrderPrintFormRetriesRetriableHTTPStatus(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\nmock"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 1)
	if _, err := client.OrderPrintForm(context.Background(), "order-ref"); err != nil {
		t.Fatalf("OrderPrintForm() error = %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestOrderPrintFormDoesNotRetryBadRequest(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, `{"success":false,"error":"unknown order"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 2)
	_, err := client.OrderPrintForm(context.Background(), "order-ref")
	if err == nil {
		t.Fatal("OrderPrintForm() error = nil")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}
