package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func adminWeightDocumentJSON(itemFields string) string {
	return fmt.Sprintf(`{
		"id":41,"version":3,"for_date":"2026-11-08T00:00:00Z",
		"customer_id":1,"customer_sale_place_id":2,
		"items":[{"id":51,"line_num":1,"product_id":10,"measure_unit_id":20,
			"quant_required":5,"quant":5%s}]
	}`, itemFields)
}

func TestAdminWeightConfirmationOptionalInputPreservesPresence(t *testing.T) {
	for _, test := range []struct {
		name       string
		itemFields string
		present    bool
		confirmed  bool
	}{
		{name: "legacy omitted marker"},
		{name: "explicit confirmed", itemFields: `,"weight_confirmed":true`, present: true, confirmed: true},
		{name: "explicit pending", itemFields: `,"weight_confirmed":false`, present: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/order/41",
				strings.NewReader(adminWeightDocumentJSON(test.itemFields)))
			document, err := decodeOrderDocumentJSON(request)
			if err != nil {
				t.Fatal(err)
			}
			if len(document.Items) != 1 {
				t.Fatalf("items = %#v", document.Items)
			}
			confirmed := document.Items[0].WeightConfirmed
			if (confirmed != nil) != test.present {
				t.Fatalf("marker presence = %t, want %t", confirmed != nil, test.present)
			}
			if confirmed != nil && *confirmed != test.confirmed {
				t.Fatalf("marker = %t, want %t", *confirmed, test.confirmed)
			}
		})
	}
}

func TestAdminWeightRequirementIsServerOwned(t *testing.T) {
	for _, value := range []string{"false", "true"} {
		request := httptest.NewRequest(http.MethodPut, "/api/order/41",
			strings.NewReader(adminWeightDocumentJSON(`,"requires_weight_confirmation":`+value)))
		if _, err := decodeOrderDocumentJSON(request); err == nil {
			t.Fatalf("client-supplied product requirement %s was accepted", value)
		}
	}
}

func maxWeightDocumentJSON(update bool, itemFields string) string {
	version := ""
	if update {
		version = `,"version":3`
	}
	return fmt.Sprintf(`{
		"customer_sale_place_id":2,"for_date":"2026-11-08",
		"items":[{"product_id":10,"quant_required":5%s}]%s
	}`, itemFields, version)
}

func TestMAXWeightConfirmationRemainsServerControlled(t *testing.T) {
	for _, update := range []bool{false, true} {
		name, method, path := "max.orders.create", http.MethodPost, "/api/max/orders"
		if update {
			name, method, path = "max.orders.update", http.MethodPut, "/api/max/orders/41"
		}
		route := findRouteByName(t, BuildRoutes(), name)
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(method, path, strings.NewReader(maxWeightDocumentJSON(update, "")))
			if update {
				request.SetPathValue("id", "41")
			}
			bound, err := route.Binder(request)
			if err != nil {
				t.Fatalf("ordinary customer input was rejected: %v", err)
			}
			if update {
				input, ok := bound.(models.MaxOrderUpdateRequest)
				if !ok || input.ID != 41 || input.Version != 3 || len(input.Items) != 1 {
					t.Fatalf("update input = %#v", bound)
				}
			} else if input, ok := bound.(models.MaxOrderSubmitRequest); !ok || len(input.Items) != 1 {
				t.Fatalf("create input = %#v", bound)
			}

			for _, field := range []string{"weight_confirmed", "requires_weight_confirmation"} {
				for _, value := range []string{"false", "true"} {
					request := httptest.NewRequest(method, path,
						strings.NewReader(maxWeightDocumentJSON(update, `,"`+field+`":`+value)))
					if update {
						request.SetPathValue("id", "41")
					}
					if _, err := route.Binder(request); err == nil {
						t.Fatalf("customer-supplied %s=%s was accepted", field, value)
					}
				}
			}
		})
	}
}
