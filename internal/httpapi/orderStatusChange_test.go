package httpapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestOrderStatusChangeJSONBinder(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/order/change-status",
		strings.NewReader(`{"order_ids":[101,102,101],"status_id":3}`))
	bound, err := orderStatusChangeJSONBinder()(request)
	if err != nil {
		t.Fatal(err)
	}
	want := models.OrderStatusChangeRequest{OrderIDs: []int{101, 102, 101}, StatusID: 3}
	if !reflect.DeepEqual(bound, want) {
		t.Fatalf("bound = %#v, want %#v", bound, want)
	}
}

func TestOrderStatusChangeJSONBinderRejectsMalformedRequests(t *testing.T) {
	for _, body := range []string{
		``,
		`{"order_ids":[101],"status_id":3,"unknown":true}`,
		`{"order_ids":[101],"status_id":3} {}`,
		`{"order_ids":[101],"status_id":"3"}`,
		`{"order_ids":[1.5],"status_id":3}`,
		strings.Repeat(" ", orderStatusChangeMaxBodySize+1),
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/order/change-status", strings.NewReader(body))
		if _, err := orderStatusChangeJSONBinder()(request); err == nil {
			t.Fatalf("invalid body of %d bytes was accepted", len(body))
		}
	}
}
