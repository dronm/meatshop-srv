package services

import (
	"context"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

func TestValidateOrderDocument(t *testing.T) {
	document := validOrderDocument()

	if err := validateOrderDocument(document, true, 0); err != nil {
		t.Fatalf("validateOrderDocument() error = %v", err)
	}
	if document.Items[0].LineNum != 1 || document.Items[1].LineNum != 2 {
		t.Fatalf("line numbers = %d, %d, want 1, 2", document.Items[0].LineNum, document.Items[1].LineNum)
	}
}

func TestValidateOrderDocumentRejectsItemIDOnCreate(t *testing.T) {
	document := validOrderDocument()
	document.Items[0].ID = 10

	if err := validateOrderDocument(document, true, 0); err == nil {
		t.Fatal("create document with item id was accepted")
	}
}

func TestValidateOrderDocumentRequiresVersionOnUpdate(t *testing.T) {
	document := validOrderDocument()
	document.ID = 12

	if err := validateOrderDocument(document, false, 12); err == nil {
		t.Fatal("update without version was accepted")
	}
}

func TestValidateOrderDocumentRejectsDuplicateItemIDs(t *testing.T) {
	document := validOrderDocument()
	document.ID = 12
	document.Version = 2
	document.Items[0].ID = 20
	document.Items[1].ID = 20

	if err := validateOrderDocument(document, false, 12); err == nil {
		t.Fatal("duplicate item ids were accepted")
	}
}

func TestValidateOrderDocumentRequiresItems(t *testing.T) {
	document := validOrderDocument()
	document.Items = nil

	if err := validateOrderDocument(document, true, 0); err == nil {
		t.Fatal("order without items was accepted")
	}
}

func TestValidateOrderDocumentNormalizes1CReferences(t *testing.T) {
	document := validOrderDocument()
	document.Ref1C = &models.Ref1c{ID: " order-1 ", Descr: " Order 1 "}
	document.ShipmentRef1C = &models.Ref1c{ID: " shipment-1 ", Descr: " Shipment 1 "}

	if err := validateOrderDocument(document, true, 0); err != nil {
		t.Fatalf("validateOrderDocument() error = %v", err)
	}
	if document.Ref1C.ID != "order-1" || document.Ref1C.Descr != "Order 1" {
		t.Fatalf("order ref = %#v", document.Ref1C)
	}
	if document.ShipmentRef1C.ID != "shipment-1" || document.ShipmentRef1C.Descr != "Shipment 1" {
		t.Fatalf("shipment ref = %#v", document.ShipmentRef1C)
	}
}

func TestValidateOrderDocumentCashOrderAllowsOptionalContactAndClearsSalePlace(t *testing.T) {
	for _, withContact := range []bool{false, true} {
		document := validOrderDocument()
		document.CashPayed = true
		if withContact {
			document.CustomerName = stringPointer("  Иван Иванов  ")
			document.CustomerPhone = stringPointer("  +7 (900) 123-45-67  ")
		}
		if err := validateOrderDocument(document, true, 0); err != nil {
			t.Fatal(err)
		}
		if document.CustomerSalePlaceID != nil {
			t.Fatalf("cash order kept its hidden sale place: %v", document.CustomerSalePlaceID)
		}
		if withContact && (*document.CustomerName != "Иван Иванов" || *document.CustomerPhone != "+7 (900) 123-45-67") {
			t.Fatalf("contact was not trimmed: %#v", document)
		}
	}
}

func TestValidateOrderDocumentRequiresCustomerForCashOrder(t *testing.T) {
	document := validOrderDocument()
	document.CashPayed = true
	document.CustomerID = 0
	if err := validateOrderDocument(document, true, 0); err == nil {
		t.Fatal("cash order without customer was accepted")
	}
}

func TestValidateOrderDocumentRegularOrderRequiresSalePlace(t *testing.T) {
	for _, salePlaceID := range []*int{nil, intPointer(0), intPointer(-1)} {
		document := validOrderDocument()
		document.CustomerSalePlaceID = salePlaceID
		if err := validateOrderDocument(document, true, 0); err == nil {
			t.Fatalf("ordinary order with sale place %v was accepted", salePlaceID)
		}
	}
}

func TestValidateOrderDocumentTrimsBlankContactsToNull(t *testing.T) {
	document := validOrderDocument()
	document.CustomerName = stringPointer(" \t ")
	document.CustomerPhone = stringPointer(" \n ")
	if err := validateOrderDocument(document, true, 0); err != nil {
		t.Fatal(err)
	}
	if document.CustomerName != nil || document.CustomerPhone != nil {
		t.Fatal("blank contacts were not normalized to null")
	}
}

type cashOrderDetailQuerier struct {
	ds.Querier
}

func (cashOrderDetailQuerier) Query(context.Context, string, ...any) (ds.Rows, error) {
	return &orderStatusTestRows{scan: func(dest ...any) error {
		*dest[0].(*int) = 101
		*dest[1].(*int64) = 2
		*dest[2].(*time.Time) = time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
		*dest[6].(*int) = 10
		*dest[7].(**models.Ref) = &models.Ref{Descr: "Заказчик"}
		*dest[16].(*bool) = true
		*dest[17].(**string) = stringPointer("Иван Иванов")
		*dest[18].(**string) = stringPointer("+79001234567")
		return nil
	}}, nil
}

func TestFetchCashOrderDetailSupportsMissingSalePlaceAndContactFields(t *testing.T) {
	detail, err := fetchOrderDetail(context.Background(), cashOrderDetailQuerier{}, 101)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.CashPayed || detail.CustomerSalePlaceID != nil || detail.CustomerSalePlace != nil {
		t.Fatalf("cash order header = %#v", detail)
	}
	if detail.CustomerID != 10 || detail.CustomerName == nil || *detail.CustomerName != "Иван Иванов" ||
		detail.CustomerPhone == nil || *detail.CustomerPhone != "+79001234567" {
		t.Fatalf("cash order contact = %#v", detail)
	}
}

func intPointer(value int) *int {
	return &value
}

func validOrderDocument() *models.OrderDocument {
	salePlaceID := 20
	return &models.OrderDocument{
		ForDate:             time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC),
		CustomerID:          10,
		CustomerSalePlaceID: &salePlaceID,
		Items: []*models.OrderDocumentItem{
			{ProductID: 30, MeasureUnitID: 40, QuantRequired: 3, Quant: 3},
			{ProductID: 31, MeasureUnitID: 40, QuantRequired: 5, Quant: 4},
		},
	}
}
