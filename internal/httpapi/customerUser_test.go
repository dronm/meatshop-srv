package httpapi

import (
	"net/http"
	"testing"

	"github.com/dronm/webapp"
)

func TestCustomerUserListRoute(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	for _, route := range routes {
		if route.Name != "customerUser.list" {
			continue
		}
		if route.Method != http.MethodGet {
			t.Fatalf("customerUser.list method = %s, want GET", route.Method)
		}
		if route.Pattern != "/api/customer-users" {
			t.Fatalf("customerUser.list pattern = %s", route.Pattern)
		}
		if route.ServiceFunc != "List" {
			t.Fatalf("customerUser.list service func = %s, want List", route.ServiceFunc)
		}
		return
	}

	t.Fatal("customerUser.list route is missing")
}
