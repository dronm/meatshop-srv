package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func TestMaxCustomerRegistrationLookupRoute(t *testing.T) {
	route := findRouteByName(t, BuildRoutes(), "max.registration.customer")

	if route.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", route.Method)
	}
	if route.Pattern != "/api/max/registration/customer" {
		t.Errorf("pattern = %q, want /api/max/registration/customer", route.Pattern)
	}
	if route.ServiceName != "MaxMiniApp" || route.ServiceFunc != "FindCustomer" {
		t.Errorf(
			"service = %s.%s, want MaxMiniApp.FindCustomer",
			route.ServiceName,
			route.ServiceFunc,
		)
	}
	if route.Binder == nil {
		t.Fatal("binder is nil")
	}
}

func TestMaxCustomerLookupBinderTrimsRequiredFields(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/max/registration/customer",
		strings.NewReader(`{"inn":"  7701234567  ","app_username":"  Иван Иванов  "}`),
	)

	bound, err := maxCustomerLookupBinder()(req)
	if err != nil {
		t.Fatalf("maxCustomerLookupBinder() error = %v", err)
	}
	input, ok := bound.(models.MaxCustomerLookupRequest)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.INN != "7701234567" || input.AppUsername != "Иван Иванов" {
		t.Fatalf("binder result = %#v", input)
	}
}

func TestMaxCustomerLookupBinderRejectsInvalidBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing inn", body: `{"app_username":"Иван Иванов"}`},
		{name: "blank inn", body: `{"inn":"  ","app_username":"Иван Иванов"}`},
		{name: "missing app username", body: `{"inn":"7701234567"}`},
		{name: "blank app username", body: `{"inn":"7701234567","app_username":"  "}`},
		{name: "legacy kpp", body: `{"inn":"7701234567","app_username":"Иван Иванов","kpp":"123456789"}`},
		{name: "trailing JSON", body: `{"inn":"7701234567","app_username":"Иван Иванов"} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/max/registration/customer",
				strings.NewReader(test.body),
			)
			if _, err := maxCustomerLookupBinder()(req); err == nil {
				t.Fatalf("body %q was accepted", test.body)
			}
		})
	}
}

func TestMaxOrderDateLimitsRoute(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	matches := 0
	for _, route := range routes {
		if route.Name != "max.orders.dateLimits" {
			continue
		}
		matches++
		if route.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", route.Method)
		}
		if route.Pattern != "/api/max/order-date-limits" {
			t.Errorf("pattern = %q, want /api/max/order-date-limits", route.Pattern)
		}
		if route.ServiceName != "MaxMiniApp" || route.ServiceFunc != "OrderDateLimits" {
			t.Errorf(
				"service = %s.%s, want MaxMiniApp.OrderDateLimits",
				route.ServiceName,
				route.ServiceFunc,
			)
		}
	}
	if matches != 1 {
		t.Fatalf("max.orders.dateLimits route count = %d, want 1", matches)
	}
}
