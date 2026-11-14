package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func TestCustomerMessageRoutes(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatal(err)
	}
	for name, permission := range map[string]string{
		"customerMessage.list":        "customerMessage.list",
		"customerMessage.detail":      "customerMessage.detail",
		"customerMessage.unreadCount": "customerMessage.list",
		"customerMessage.markRead":    "customerMessage.markRead",
	} {
		route := findRouteByName(t, routes, name)
		if route.Permission != permission || route.ServiceName != "CustomerMessage" {
			t.Fatalf("route %s: %#v", name, route)
		}
	}
	markRead := findRouteByName(t, routes, "customerMessage.markRead")
	for _, body := range []string{
		`{"message_ids":[1],"user_id":999}`, `{"message_ids":[1]} {}`, `null {}`,
	} {
		if _, err := markRead.Binder(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))); err == nil {
			t.Fatalf("accepted body %s", body)
		}
	}
	input, err := markRead.Binder(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"message_ids":[1,2]}`)))
	if err != nil || len(input.(models.CustomerMessageMarkReadRequest).MessageIDs) != 2 {
		t.Fatalf("bound input = %#v, err = %v", input, err)
	}
}
