package services

import (
	"testing"
	"time"

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

func validOrderDocument() *models.OrderDocument {
	return &models.OrderDocument{
		ForDate:             time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC),
		CustomerID:          10,
		CustomerSalePlaceID: 20,
		Items: []*models.OrderDocumentItem{
			{ProductID: 30, MeasureUnitID: 40, QuantRequired: 3, Quant: 3},
			{ProductID: 31, MeasureUnitID: 40, QuantRequired: 5, Quant: 4},
		},
	}
}
