package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func TestOrderDocumentRoutesReplaceGeneratedEndpoints(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	want := map[string]struct {
		method      string
		pattern     string
		serviceFunc string
	}{
		"order.create":   {http.MethodPost, "/api/order", "Create"},
		"order.detail":   {http.MethodGet, "/api/order/{id}", "DocumentDetail"},
		"order.update":   {http.MethodPut, "/api/order/{id}", "Update"},
		"order.create1c": {http.MethodPost, "/api/order/{id}/create-1c", "Create1C"},
		"order.print1c":  {http.MethodGet, "/api/order/{id}/print-1c", ""},
		"order.delete":   {http.MethodDelete, "/api/order/{id}", "Delete"},
		"order.list":     {http.MethodGet, "/api/order", "List"},
	}

	found := make(map[string]int, len(want))
	for _, route := range routes {
		if strings.HasPrefix(route.Name, "orderItem.") ||
			strings.HasPrefix(route.Pattern, "/api/order-items") {
			t.Errorf("standalone order item route is still exposed: %s %s", route.Name, route.Pattern)
		}

		expected, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name]++
		if route.Method != expected.method || route.Pattern != expected.pattern || route.ServiceFunc != expected.serviceFunc {
			t.Errorf(
				"route %s = %s %s -> %s, want %s %s -> %s",
				route.Name,
				route.Method,
				route.Pattern,
				route.ServiceFunc,
				expected.method,
				expected.pattern,
				expected.serviceFunc,
			)
		}
		if route.Name == "order.print1c" {
			if route.Permission != "order.print1c" {
				t.Errorf("order.print1c permission = %q", route.Permission)
			}
			if route.Handler == nil {
				t.Error("order.print1c handler is nil")
			}
		}
	}

	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}

func TestOrderDocumentBinder(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{
		"for_date":"2026-08-25T00:00:00Z",
		"customer_id":10,
		"customer_sale_place_id":20,
		"items":[{
			"product_id":30,
			"measure_unit_id":40,
			"quant_required":2.5,
			"quant":2
		}]
	}`))

	bound, err := orderDocumentJSONBinder()(req)
	if err != nil {
		t.Fatalf("order binder error = %v", err)
	}
	document, ok := bound.(*models.OrderDocument)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if document.CustomerID != 10 || len(document.Items) != 1 || document.Items[0].QuantRequired != 2.5 {
		t.Fatalf("binder result = %#v", document)
	}
	if got := document.ForDate.Format(time.RFC3339); got != "2026-08-25T00:00:00Z" {
		t.Fatalf("order date = %s", got)
	}
}

func TestOrderDocumentUpdateBinderUsesPathID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/order/42", strings.NewReader(`{
		"version":3,
		"for_date":"2026-08-25T00:00:00Z",
		"customer_id":10,
		"customer_sale_place_id":20,
		"items":[{
			"product_id":30,
			"measure_unit_id":40,
			"quant_required":2,
			"quant":2
		}]
	}`))
	req.SetPathValue("id", "42")

	bound, err := orderDocumentUpdateBinder()(req)
	if err != nil {
		t.Fatalf("update binder error = %v", err)
	}
	input, ok := bound.(models.UpdateOrderDocumentRequest)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.ID != 42 || input.Document.Version != 3 {
		t.Fatalf("binder result = %#v", input)
	}
}

func TestOrderDocumentBinderRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, body := range []string{
		`{"for_date":"2026-08-25T00:00:00Z","unknown":true}`,
		`{} {}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(body))
		if _, err := orderDocumentJSONBinder()(req); err == nil {
			t.Fatalf("body %q was accepted", body)
		}
	}
}
