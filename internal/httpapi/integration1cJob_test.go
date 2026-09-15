package httpapi

import (
	"net/http"
	"testing"

	"github.com/dronm/webapp"
)

func TestIntegration1CJobRoute(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	var matches int
	for _, route := range routes {
		if route.Name != "integration1c.jobs.list" {
			continue
		}

		matches++
		if route.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", route.Method)
		}
		if route.Pattern != "/api/integration-1c/jobs" {
			t.Errorf("pattern = %q", route.Pattern)
		}
		if route.Permission != "integration1c.jobs.list" {
			t.Errorf("permission = %q", route.Permission)
		}
		if route.ServiceFunc != "List" {
			t.Errorf("service func = %q, want List", route.ServiceFunc)
		}
	}

	if matches != 1 {
		t.Fatalf("integration1c.jobs.list route count = %d, want 1", matches)
	}
}
