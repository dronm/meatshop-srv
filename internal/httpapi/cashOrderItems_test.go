package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCashOrderBinderAcceptsManualDecimalMoney(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{
		"for_date":"2026-11-16T00:00:00Z","customer_id":10,"cash_payed":true,
		"items":[{"product_id":30,"measure_unit_id":40,"quant":2.375,
			"price":"12.123456","amount":"50.01"}]
	}`))
	document, err := decodeOrderDocumentJSON(request)
	if err != nil {
		t.Fatal(err)
	}
	item := document.Items[0]
	if !document.CashPayed || item.Price == nil || *item.Price != "12.123456" ||
		item.Amount == nil || *item.Amount != "50.01" {
		t.Fatalf("manual monetary values were lost during binding: %#v", item)
	}
}
