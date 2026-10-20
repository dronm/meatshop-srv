package services

import (
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestRemovedOrderItemSnapshots(t *testing.T) {
	old := map[int]orderItemSnapshot{
		10: {LineNum: 1, ProductID: 100, ProductName: "Мясо", QuantRequired: 5, Quant: 5},
		20: {LineNum: 2, ProductID: 100, ProductName: "Мясо", QuantRequired: 8, Quant: 6},
		30: {LineNum: 3, ProductID: 200, ProductName: "Фарш", QuantRequired: 2, Quant: 0},
	}
	// Another line for the same product survives. Detection must use line IDs.
	items := []*models.OrderDocumentItem{{ID: 20, ProductID: 100, QuantRequired: 8, Quant: 6}}
	removed := removedOrderItemSnapshots(old, items)
	if len(removed) != 2 || removed[0] != old[10] || removed[1] != old[30] {
		t.Fatalf("unexpected removed lines or order: %#v", removed)
	}
	if changed := changedConfirmedQuantityItemIDs(old, items); len(changed) != 0 {
		t.Fatalf("deletion-only edit should not require a quantity mismatch: %v", changed)
	}
	// On the next save, the deleted lines are absent from the saved snapshot.
	if repeated := removedOrderItemSnapshots(map[int]orderItemSnapshot{20: old[20]}, items); len(repeated) != 0 {
		t.Fatalf("unchanged save detected repeated removals: %#v", repeated)
	}
}

func TestOrderChangeNotificationIncludesRemovedLines(t *testing.T) {
	detail := &models.OrderDetail{ID: 17}
	data := orderNotificationDataFromDetail(detail, false, nil, []orderItemSnapshot{
		{ProductID: 100, ProductName: "Мясо", QuantRequired: 5, Quant: 5},
		{ProductID: 200, QuantRequired: 2, Quant: 0},
	})
	// Existing/custom templates using ChangedItems also receive deletion text.
	text, err := renderNotificationTemplate("order_changed",
		`Ваш заказ №{{.OrderID}} изменён.{{if .ChangedItems}}\n{{.ChangedItems}}{{end}}`, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Ваш заказ №17 изменён.",
		"• Мясо: позиция удалена (заказано: 5; подтверждено: 5)",
		"• 200: позиция удалена (заказано: 2; подтверждено: 0)",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("notification omits %q: %s", expected, text)
		}
	}
}

func TestOrderChangeNotificationCombinesQuantityAndDeletion(t *testing.T) {
	detail := &models.OrderDetail{ID: 17, Items: []*models.OrderDetailItem{
		{ID: 10, ProductID: 100, QuantRequired: 5, Quant: 3},
	}}
	data := orderNotificationDataFromDetail(detail, true, map[int]struct{}{10: {}}, []orderItemSnapshot{
		{ProductID: 200, ProductName: "Фарш", QuantRequired: 2, Quant: 2},
	})
	if !data.StatusChanged || !strings.Contains(data.ChangedItems, "• 100: 5 → 3") ||
		!strings.Contains(data.ChangedItems, "• Фарш: позиция удалена") {
		t.Fatalf("combined notification lost a change: %#v", data)
	}
}

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
