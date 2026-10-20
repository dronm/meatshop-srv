package httpapi

import (
	"strings"
	"testing"

	"github.com/dronm/webapp"
)

func TestIntegration1CAdminRoutes(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"health": "GET", "status": "GET", "start": "POST", "stop": "POST"}
	for _, route := range routes {
		command := strings.TrimPrefix(route.Name, "integration1c.admin.")
		method, ok := want[command]
		if !ok {
			continue
		}
		if route.Method != method || route.Pattern != "/api/integration-1c/"+command ||
			route.Permission != "integration1c.admin" || route.ServiceName != "Integration1C" ||
			route.ServiceFunc != strings.ToUpper(command[:1])+command[1:] {
			t.Errorf("incorrect admin route: %+v", route)
		}
		delete(want, command)
	}
	if len(want) > 0 {
		t.Fatalf("missing admin routes: %v", want)
	}
}
