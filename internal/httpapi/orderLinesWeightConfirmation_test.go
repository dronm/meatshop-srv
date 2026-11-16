package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestOrderLinesWeightConfirmationBinderAcceptsExplicitTrueAndFalse(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		r := httptest.NewRequest(http.MethodPost, "/api/order/set-lines-weight-confirmed",
			strings.NewReader(`{"item_ids":[3,1,3],"weight_confirmed":`+value+`}`))
		bound, err := orderLinesWeightConfirmationJSONBinder()(r)
		if err != nil {
			t.Fatal(err)
		}
		input := bound.(models.OrderLinesWeightConfirmationRequest)
		if input.WeightConfirmed == nil || *input.WeightConfirmed != (value == "true") || len(input.ItemIDs) != 3 {
			t.Fatalf("explicit %s not retained: %#v", value, input)
		}
	}
}

func TestOrderLinesWeightConfirmationBinderRejectsMissingInvalidAndUnknownValues(t *testing.T) {
	for _, body := range []string{
		``, `null`, `{}`, `{"item_ids":[1]}`,
		`{"item_ids":[1],"weight_confirmed":null}`,
		`{"item_ids":[1],"weight_confirmed":"false"}`,
		`{"item_ids":[1],"weight_confirmed":0}`,
		`{"item_ids":[1],"weight_confirmed":true,"unknown":1}`,
		`{"item_ids":[1],"weight_confirmed":true} {}`,
		`{"item_ids":[1.5],"weight_confirmed":true}`,
		strings.Repeat(" ", orderLineIDsMaxBodySize+1),
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/order/set-lines-weight-confirmed", strings.NewReader(body))
		if _, err := orderLinesWeightConfirmationJSONBinder()(r); err == nil {
			t.Fatalf("invalid body accepted: %.100s", body)
		}
	}
	if _, err := orderLinesWeightConfirmationJSONBinder()(nil); err == nil {
		t.Fatal("nil request was accepted")
	}
}
