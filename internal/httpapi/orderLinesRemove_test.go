package httpapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestOrderLinesRemoveJSONBinderUsesItemIDs(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/order/remove-lines",
		strings.NewReader(`{"item_ids":[301,302,301]}`))
	bound, err := orderLinesRemoveJSONBinder()(request)
	if err != nil {
		t.Fatal(err)
	}
	want := models.OrderLinesRemoveRequest{ItemIDs: []int{301, 302, 301}}
	if !reflect.DeepEqual(bound, want) {
		t.Fatalf("bound = %#v, want %#v", bound, want)
	}
}

func TestOrderLinesRemoveJSONBinderRejectsMalformedRequests(t *testing.T) {
	for _, body := range []string{
		``,
		`{"item_ids":[301],"order_ids":[101]}`,
		`{"item_ids":[301]} {}`,
		`{"item_ids":[301]} garbage`,
		`{"item_ids":[1.5]}`,
		`{"item_ids":["301"]}`,
		`{"item_ids":301}`,
		`{"item_ids":[301]`,
		strings.Repeat(" ", orderLinesRemoveMaxBodySize+1),
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/order/remove-lines", strings.NewReader(body))
		if _, err := orderLinesRemoveJSONBinder()(request); err == nil {
			t.Fatalf("invalid body of %d bytes was accepted: %.100s", len(body), body)
		}
	}
	for _, request := range []*http.Request{nil, {}} {
		if _, err := orderLinesRemoveJSONBinder()(request); err == nil {
			t.Fatal("missing request body was accepted")
		}
	}
}
