package services

import (
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestChangedConfirmedQuantityItemIDs(t *testing.T) {
	old := map[int]orderItemSnapshot{
		10: {QuantRequired: 5, Quant: 5},
		20: {QuantRequired: 8, Quant: 6},
	}
	items := []*models.OrderDocumentItem{
		{ID: 10, QuantRequired: 5, Quant: 3}, // newly changed mismatch
		{ID: 20, QuantRequired: 8, Quant: 6}, // old mismatch, unchanged
		{ID: 30, QuantRequired: 4, Quant: 4}, // new but equal
		{ID: 40, QuantRequired: 7, Quant: 0}, // new mismatch, including zero
	}

	changed := changedConfirmedQuantityItemIDs(old, items)
	if _, ok := changed[10]; !ok {
		t.Fatal("changed item 10 was not detected")
	}
	if _, ok := changed[40]; !ok {
		t.Fatal("new mismatched item 40 was not detected")
	}
	if _, ok := changed[20]; ok {
		t.Fatal("unchanged old mismatch should not trigger another notification")
	}
	if _, ok := changed[30]; ok {
		t.Fatal("equal quantities should not trigger a notification")
	}
}

func TestRenderNotificationTemplateResubmitted(t *testing.T) {
	body := `{{if .Resubmitted}}resent{{else}}new{{end}} order {{.OrderID}}`
	text, err := renderNotificationTemplate("test", body, orderNotificationData{
		OrderID:     17,
		Resubmitted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "resent order 17" {
		t.Fatalf("unexpected rendered text: %q", text)
	}
}
