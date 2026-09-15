package integration1c

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestPrintCommandsUseBinDataAndReturnPDF(t *testing.T) {
	pdf := []byte("%PDF-1.7\nmock pdf\n%%EOF\n")

	tests := []struct {
		name     string
		command  string
		fileName string
		call     func(*Client) (BinaryResponse, error)
	}{
		{
			name:     "order",
			command:  CommandPrintOrder,
			fileName: "orders-1c.pdf",
			call: func(client *Client) (BinaryResponse, error) {
				return client.PrintOrder(context.Background(), []int{101, 102})
			},
		},
		{
			name:     "shipment",
			command:  CommandPrintShipment,
			fileName: "shipments-1c.pdf",
			call: func(client *Client) (BinaryResponse, error) {
				return client.PrintShipment(context.Background(), []int{101, 102})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", r.Method)
				}
				if r.URL.Path != "/bin-data" {
					t.Fatalf("path = %s, want /bin-data", r.URL.Path)
				}

				var request struct {
					Command string         `json:"command"`
					Params  OrderIDsParams `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if request.Command != test.command {
					t.Fatalf("command = %q, want %q", request.Command, test.command)
				}
				if !reflect.DeepEqual(request.Params.OrderIDs, []int{101, 102}) {
					t.Fatalf("order_ids = %v", request.Params.OrderIDs)
				}

				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition", `attachment; filename="`+test.fileName+`"`)
				_, _ = w.Write(pdf)
			}))
			defer server.Close()

			client := newTestClient(t, server.URL+"/execute", 0)
			result, err := test.call(client)
			if err != nil {
				t.Fatalf("print command error = %v", err)
			}
			if string(result.Body) != string(pdf) {
				t.Fatalf("body = %q, want %q", result.Body, pdf)
			}
			if result.ContentType != "application/pdf" {
				t.Fatalf("content type = %q", result.ContentType)
			}
			if result.FileName != test.fileName {
				t.Fatalf("file name = %q", result.FileName)
			}
		})
	}
}

func TestPrintOrderAcceptsOctetStreamWhenBodyIsPDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("%PDF-1.4\nmock"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	result, err := client.PrintOrder(context.Background(), []int{101})
	if err != nil {
		t.Fatalf("PrintOrder() error = %v", err)
	}
	if result.ContentType != "application/pdf" {
		t.Fatalf("content type = %q", result.ContentType)
	}
}

func TestPrintShipmentRejectsNonPDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not a pdf"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 0)
	if _, err := client.PrintShipment(context.Background(), []int{101}); err == nil {
		t.Fatal("PrintShipment() error = nil")
	}
}

func TestPrintOrderRetriesRetriableHTTPStatus(t *testing.T) {
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
	if _, err := client.PrintOrder(context.Background(), []int{101}); err != nil {
		t.Fatalf("PrintOrder() error = %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestPrintOrderDoesNotRetryBadRequest(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, `{"success":false,"error":"unknown order"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 2)
	_, err := client.PrintOrder(context.Background(), []int{101})
	if err == nil {
		t.Fatal("PrintOrder() error = nil")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestBatchCommandsRejectInvalidOrderIDs(t *testing.T) {
	tests := []struct {
		name     string
		orderIDs []int
	}{
		{name: "empty"},
		{name: "zero", orderIDs: []int{101, 0}},
		{name: "negative", orderIDs: []int{-1}},
		{name: "duplicate", orderIDs: []int{101, 101}},
	}

	client := newTestClient(t, "http://127.0.0.1:1", 0)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.PrintOrder(context.Background(), test.orderIDs); err == nil {
				t.Fatal("PrintOrder() error = nil")
			}
			if _, err := client.PrintShipment(context.Background(), test.orderIDs); err == nil {
				t.Fatal("PrintShipment() error = nil")
			}
			if _, err := client.CreateShipments(context.Background(), test.orderIDs); err == nil {
				t.Fatal("CreateShipments() error = nil")
			}
		})
	}
}
