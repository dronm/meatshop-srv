package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/webapp"
)

func TestOrderCalendarDayRoutes(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatalf("ValidateRoutes() error = %v", err)
	}

	// Building the ServeMux exercises Go's semantic route-conflict detection,
	// including patterns that differ only in the path-variable name.
	webapp.BuildRouter(&webapp.AppContext{}, routes)

	want := map[string]struct {
		method      string
		pattern     string
		serviceFunc string
	}{
		"orderCalendarDay.create": {http.MethodPost, "/api/order-calendar-days", "Create"},
		"orderCalendarDay.list":   {http.MethodGet, "/api/order-calendar-days", "List"},
		"orderCalendarDay.detail": {http.MethodGet, "/api/order-calendar-days/{id}", "Detail"},
		"orderCalendarDay.update": {http.MethodPatch, "/api/order-calendar-days/{id}", "Update"},
		"orderCalendarDay.delete": {http.MethodDelete, "/api/order-calendar-days/{id}", "Delete"},
	}

	found := make(map[string]int, len(want))
	for _, route := range routes {
		expected, ok := want[route.Name]
		if !ok {
			continue
		}

		found[route.Name]++
		if route.Method != expected.method ||
			route.Pattern != expected.pattern ||
			route.ServiceName != "OrderCalendarDay" ||
			route.ServiceFunc != expected.serviceFunc {
			t.Errorf(
				"route %s = %s %s -> %s.%s, want %s %s -> OrderCalendarDay.%s",
				route.Name,
				route.Method,
				route.Pattern,
				route.ServiceName,
				route.ServiceFunc,
				expected.method,
				expected.pattern,
				expected.serviceFunc,
			)
		}
		if route.Permission != route.Name {
			t.Errorf("route %s permission = %q, want %q", route.Name, route.Permission, route.Name)
		}
		if route.Binder == nil {
			t.Errorf("route %s binder is nil", route.Name)
		}
	}

	for name := range want {
		if found[name] != 1 {
			t.Errorf("route %s count = %d, want 1", name, found[name])
		}
	}
}

func TestOrderCalendarDayCreateBinderDecodesCalendarDate(t *testing.T) {
	route := findRouteByName(t, BuildRoutes(), "orderCalendarDay.create")
	req := httptest.NewRequest(http.MethodPost, route.Pattern, strings.NewReader(`{
		"calendar_date":"2026-09-15T00:00:00Z",
		"is_holiday":false,
		"name":"Working-day override"
	}`))

	bound, err := route.Binder(req)
	if err != nil {
		t.Fatalf("create binder error = %v", err)
	}
	input, ok := bound.(modelbind.ModelInput[*models.OrderCalendarDay])
	if !ok {
		t.Fatalf("create binder result type = %T", bound)
	}
	if input.Model == nil {
		t.Fatal("create binder model is nil")
	}
	wantDate := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
	if !input.Model.CalendarDate.Equal(wantDate) || input.Model.IsHoliday {
		t.Fatalf("create binder model = %#v", input.Model)
	}
	if input.Model.Name == nil || *input.Model.Name != "Working-day override" {
		t.Fatalf("create binder name = %#v", input.Model.Name)
	}
}

func TestOrderCalendarDayPathAndUpdateBinders(t *testing.T) {
	routes := BuildRoutes()

	for _, name := range []string{"orderCalendarDay.detail", "orderCalendarDay.delete"} {
		route := findRouteByName(t, routes, name)
		req := httptest.NewRequest(route.Method, "/api/order-calendar-days/42", nil)
		req.SetPathValue("id", "42")

		bound, err := route.Binder(req)
		if err != nil {
			t.Fatalf("%s binder error = %v", name, err)
		}
		if id, ok := bound.(int); !ok || id != 42 {
			t.Fatalf("%s binder result = %#v (%T)", name, bound, bound)
		}
	}

	route := findRouteByName(t, routes, "orderCalendarDay.update")
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/order-calendar-days/42",
		strings.NewReader(`{"calendar_date":"2026-09-16T00:00:00Z","is_holiday":true,"name":"Holiday"}`),
	)
	req.SetPathValue("id", "42")

	bound, err := route.Binder(req)
	if err != nil {
		t.Fatalf("update binder error = %v", err)
	}
	input, ok := bound.(webapp.UpdateByKeysInput[*models.OrderCalendarDayKey, *models.OrderCalendarDay])
	if !ok {
		t.Fatalf("update binder result type = %T", bound)
	}
	if input.Keys == nil || input.Keys.ID != 42 {
		t.Fatalf("update binder keys = %#v", input.Keys)
	}
	wantDate := time.Date(2026, time.September, 16, 0, 0, 0, 0, time.UTC)
	if input.Input.Model == nil ||
		!input.Input.Model.CalendarDate.Equal(wantDate) ||
		!input.Input.Model.IsHoliday {
		t.Fatalf("update binder model = %#v", input.Input.Model)
	}
}

func findRouteByName(t *testing.T, routes []webapp.Route, name string) webapp.Route {
	t.Helper()

	for _, route := range routes {
		if route.Name == name {
			return route
		}
	}

	t.Fatalf("route %s is missing", name)
	return webapp.Route{}
}
