package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/webapp"
	"github.com/dronm/webapp/constants"
)

func TestConstantRoutes(t *testing.T) {
	routes := BuildRoutes()
	if err := webapp.ValidateRoutes(routes); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		method string
		path   string
		call   string
	}{
		{"constant.list", http.MethodGet, "/api/constants", "List"},
		{"constant.detail", http.MethodGet, "/api/constants/{id}", "Detail"},
		{"constant.get", http.MethodPost, "/api/constants/get", "Get"},
		{"constant.set", http.MethodPost, "/api/constants/set", "Set"},
	} {
		route := findRouteByName(t, routes, test.name)
		if route.Method != test.method || route.Pattern != test.path || route.Permission != test.name || route.ServiceName != "Constant" || route.ServiceFunc != test.call || route.Binder == nil {
			t.Fatalf("incorrect constant route: %#v", route)
		}
	}
	get := findRouteByName(t, routes, "constant.get")
	input, err := get.Binder(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ids":[12,13]}`)))
	if err != nil || len(input.(constants.GetInput).IDs) != 2 {
		t.Fatalf("get binder: input = %#v, err = %v", input, err)
	}
	set := findRouteByName(t, routes, "constant.set")
	input, err = set.Binder(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"items":[{"id":12,"value":{"keys":{"id":17},"descr":"Покупатель"},"expected_version":3}]}`)))
	if err != nil {
		t.Fatal(err)
	}
	changes := input.(constants.SetInput).Items
	if len(changes) != 1 || changes[0].ExpectedVersion == nil || *changes[0].ExpectedVersion != 3 || string(changes[0].Value) != `{"keys":{"id":17},"descr":"Покупатель"}` {
		t.Fatalf("set binder: %#v", changes)
	}
	for _, test := range []struct {
		name string
		body string
	}{
		{"constant.get", `null`},
		{"constant.get", `{"ids":[1],"user_id":99}`},
		{"constant.get", `{"ids":[1]} {}`},
		{"constant.set", `{"items":[{"id":1,"value":null,"unknown":true}]}`},
	} {
		route := findRouteByName(t, routes, test.name)
		if _, err := route.Binder(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))); err == nil {
			t.Fatalf("%s accepted invalid body %s", test.name, test.body)
		}
	}
}
