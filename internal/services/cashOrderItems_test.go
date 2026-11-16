package services

import (
	"math"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestCashOrderQuantityUsesActualQuantityOnCreateAndUpdate(t *testing.T) {
	for _, create := range []bool{true, false} {
		document := validOrderDocument()
		document.CashPayed = true
		pathID := 0
		if !create {
			pathID, document.ID, document.Version = 41, 41, 2
		}
		document.Items[0].QuantRequired = 0
		document.Items[0].Quant = 2.375
		if err := validateOrderDocument(document, create, pathID); err != nil {
			t.Fatal(err)
		}
		for _, item := range document.Items {
			if item.QuantRequired != item.Quant {
				t.Fatalf("create=%t: requested=%v, actual=%v", create, item.QuantRequired, item.Quant)
			}
		}
	}
}

func TestRegularOrderKeepsRequestedQuantityAndAllowsRemovedLines(t *testing.T) {
	document := validOrderDocument()
	document.Items[0].Quant = 0
	if err := validateOrderDocument(document, true, 0); err != nil {
		t.Fatal(err)
	}
	if document.Items[0].QuantRequired != 3 || document.Items[1].QuantRequired != 5 {
		t.Fatal("ordinary requested quantities were changed")
	}
}

func TestCashOrderRejectsNonpositiveAndNonfiniteActualQuantity(t *testing.T) {
	for _, quant := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		document := validOrderDocument()
		document.CashPayed = true
		document.Items[0].Quant = quant
		if err := validateOrderDocument(document, true, 0); err == nil {
			t.Fatalf("cash quantity %v was accepted", quant)
		}
	}
}

func TestCashOrderValidatesManualDecimalMoneyAndKeepsAmountOverride(t *testing.T) {
	document := validOrderDocument()
	document.CashPayed = true
	document.Items[0].Price = stringPointer(" 12.123456 ")
	document.Items[0].Amount = stringPointer(" 50.01 ")
	if err := validateOrderDocument(document, true, 0); err != nil {
		t.Fatal(err)
	}
	if *document.Items[0].Price != "12.123456" || *document.Items[0].Amount != "50.01" {
		t.Fatalf("manual money was not retained: %#v", document.Items[0])
	}
	for _, test := range []struct {
		field string
		value string
	}{
		{"price", "-1"},
		{"price", "NaN"},
		{"price", "1.0000001"},
		{"price", "10000000000000"},
		{"amount", "-1"},
		{"amount", "1.001"},
		{"amount", "10000000000000"},
	} {
		t.Run(test.field+"="+test.value, func(t *testing.T) {
			document := validOrderDocument()
			document.CashPayed = true
			item := document.Items[0]
			if test.field == "price" {
				item.Price = &test.value
			} else {
				item.Amount = &test.value
			}
			if err := validateOrderDocument(document, true, 0); err == nil {
				t.Fatalf("invalid manual money was accepted: %#v", item)
			}
		})
	}
}

func TestCashOrderAllowsOmittedManualMoney(t *testing.T) {
	document := validOrderDocument()
	document.CashPayed = true
	document.Items[0].Price = stringPointer(" \t ")
	if err := validateOrderDocument(document, true, 0); err != nil {
		t.Fatal(err)
	}
	if document.Items[0].Price != nil || document.Items[0].Amount != nil {
		t.Fatalf("omitted money should be optional: %#v", document.Items[0])
	}
	// New decimal fields do not grant ordinary orders a manual price write path.
	if err := syncCashOrderItemPrices(t.Context(), nil, &models.OrderDocument{}); err != nil {
		t.Fatal(err)
	}
}
