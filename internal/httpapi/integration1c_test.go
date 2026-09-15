package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dronm/webapp"
)

func TestIntegration1CRoutes(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	want := map[string]struct {
		pattern     string
		serviceFunc string
	}{
		"integration1c.nomenclature.complete": {
			pattern:     "/api/integration-1c/nomenclature",
			serviceFunc: "CompleteNomenclature",
		},
		"integration1c.counterparty.complete": {
			pattern:     "/api/integration-1c/counterparties",
			serviceFunc: "CompleteCounterparties",
		},
	}

	found := make(map[string]bool, len(want))
	for _, route := range routes {
		expected, ok := want[route.Name]
		if !ok {
			continue
		}
		found[route.Name] = true
		if route.Method != http.MethodGet {
			t.Errorf("route %s method = %s, want GET", route.Name, route.Method)
		}
		if route.Pattern != expected.pattern {
			t.Errorf("route %s pattern = %s, want %s", route.Name, route.Pattern, expected.pattern)
		}
		if route.ServiceFunc != expected.serviceFunc {
			t.Errorf("route %s service func = %s, want %s", route.Name, route.ServiceFunc, expected.serviceFunc)
		}
	}

	for name := range want {
		if !found[name] {
			t.Errorf("route %s is missing", name)
		}
	}
}

func TestIntegration1CNameBinder(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/integration-1c/nomenclature?name=%D0%BC%D1%8F%D1%81", nil)

	bound, err := integration1CNameBinder()(req)
	if err != nil {
		t.Fatalf("binder error = %v", err)
	}
	name, ok := bound.(string)
	if !ok {
		t.Fatalf("binder result type = %T, want string", bound)
	}
	if name != "мяс" {
		t.Fatalf("binder result = %q, want %q", name, "мяс")
	}
}

func TestIntegration1CNameBinderRejectsEmptyName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/integration-1c/nomenclature?name=%20%20", nil)
	if _, err := integration1CNameBinder()(req); err == nil {
		t.Fatal("empty name was accepted")
	}
}
