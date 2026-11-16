package services

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/dronm/meatshop/internal/models"
)

func productionTestRef(id int, name string) *models.Ref {
	ref := &models.Ref{Descr: name}
	ref.Keys.ID = id
	return ref
}

func TestProductionTotalsKeepUnitsAndExactDecimals(t *testing.T) {
	orders := []*models.ProductionOrderRow{{Items: []models.ProductionOrderItem{
		{ProductID: 1, ProductName: "Филе", MeasureUnitID: 1, MeasureUnitName: "кг", Quant: "0.1000"},
		{ProductID: 1, ProductName: "Филе", MeasureUnitID: 1, MeasureUnitName: "кг", Quant: "0.2000"},
		{ProductID: 1, ProductName: "Филе", MeasureUnitID: 2, MeasureUnitName: "кор", Quant: "2.0000"},
		{ProductID: 2, ProductName: "Яйцо", MeasureUnitID: 2, MeasureUnitName: "кор", Quant: "3.0000"},
		{ProductID: 3, ProductName: "Другое", MeasureUnitID: 3, MeasureUnitName: "шт", Quant: "999999999999999.9999"},
	}}}
	groups, err := productionTotals(orders)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 || groups[0].UnitName != "кг" || groups[1].UnitName != "кор" || groups[2].UnitName != "шт" {
		t.Fatalf("unexpected grouping: %#v", groups)
	}
	if got := productionRatString(groups[0].Quant); got != "0,3" {
		t.Fatalf("exact decimal total=%q", got)
	}
	if got := productionRatString(groups[1].Quant); got != "5" {
		t.Fatalf("box total=%q", got)
	}
	if got := productionRatString(groups[2].Quant); got != "999999999999999,9999" {
		t.Fatalf("large decimal total=%q", got)
	}
}

func TestProductionMessagesSplitUnicodeAndPreserveDetailsBeforeTotals(t *testing.T) {
	items := make([]models.ProductionOrderItem, 240)
	for i := range items {
		items[i] = models.ProductionOrderItem{ProductID: i + 1, ProductName: strings.Repeat("Филе", 20), MeasureUnitID: 1, MeasureUnitName: "кг", Quant: "1.2500"}
	}
	messages, err := productionMessages(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), []*models.ProductionOrderRow{{Customer: productionTestRef(1, "Заказчик"), CustomerSalePlace: productionTestRef(2, "Точка"), Items: items}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) < 3 {
		t.Fatalf("long list was not split: %d", len(messages))
	}
	seenTotals := false
	for _, message := range messages {
		if !utf8.ValidString(message) || utf8.RuneCountInString(message) > 4000 {
			t.Fatalf("invalid or oversized message: %d", utf8.RuneCountInString(message))
		}
		if strings.HasPrefix(message, "Общее:") {
			seenTotals = true
		} else if seenTotals {
			t.Fatal("details appeared after totals")
		}
	}
	if !seenTotals {
		t.Fatal("totals missing")
	}
	if !strings.Contains(strings.Join(messages, "\n"), "Итого: 300 кг") {
		t.Fatal("totals incorrect or missing")
	}
	oversize := splitProductionMessage("Заявка", []string{strings.Repeat("Я", 12000)}, 4000)
	for _, part := range oversize {
		if utf8.RuneCountInString(part) > 4000 || !utf8.ValidString(part) {
			t.Fatal("single long line was split incorrectly")
		}
	}
}

func TestProductionPrintUsesItemMarkingAndEscapesLiveComments(t *testing.T) {
	comment := "<script>alert('x')</script>"
	doc := &models.ProductionTransferDocument{ID: 42, DocumentDate: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), ForDate: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), Orders: []*models.ProductionOrderRow{{Customer: productionTestRef(1, "Заказчик"), CustomerSalePlace: productionTestRef(2, "Точка"), CommentCustomer: &comment, Items: []models.ProductionOrderItem{
		{ProductID: 1, ProductName: "Первый", MeasureUnitID: 1, MeasureUnitName: "кг", Quant: "50.0000", UseMarking: false},
		{ProductID: 2, ProductName: "Второй", MeasureUnitID: 1, MeasureUnitName: "кг", Quant: "20.0000", UseMarking: true},
	}}}}
	output, err := renderProductionTransfer(doc)
	if err != nil {
		t.Fatal(err)
	}
	html := string(output)
	for _, required := range []string{"Заявка на 14.11.26", "Заказчик -- Точка", "Первый — 50 кг", "Второй — 20 кг", `class="mark">Нет`, `class="mark">Да`, "&lt;script&gt;"} {
		if !strings.Contains(html, required) {
			t.Errorf("print missing %q", required)
		}
	}
	if strings.Contains(html, "<script>") {
		t.Fatal("unescaped comment in HTML")
	}
}

func TestProductionLockUnionMayExceedDocumentLimit(t *testing.T) {
	ids := make([]int, 0, 2000)
	for i := 1000; i >= 1; i-- {
		ids = append(ids, i)
	}
	for i := 500; i <= 1499; i++ {
		ids = append(ids, i)
	}
	got := productionLockIDs(ids)
	if len(got) != 1499 || got[0] != 1 || got[len(got)-1] != 1499 {
		t.Fatalf("lock union lost orders: len=%d", len(got))
	}
	for i, id := range got {
		if id != i+1 {
			t.Fatalf("lock union is not sorted/deduplicated at %d: %d", i, id)
		}
	}
}

func TestProductionSelectionAndDates(t *testing.T) {
	ids, err := productionOrderIDs([]int{3, 3, 1}, false)
	if err != nil || len(ids) != 2 || ids[0] != 3 || ids[1] != 1 {
		t.Fatalf("selection=%v err=%v", ids, err)
	}
	if _, err := productionOrderIDs(nil, false); err == nil {
		t.Fatal("empty new document accepted")
	}
	if _, err := productionOrderIDs(nil, true); err != nil {
		t.Fatal("empty existing document rejected")
	}
	if _, err := productionDate("not-a-date"); err == nil {
		t.Fatal("invalid date accepted")
	}
	date, err := productionDate("2026-11-14T00:00:00.000Z")
	if err != nil || date.Format("2006-01-02") != "2026-11-14" {
		t.Fatalf("date=%v err=%v", date, err)
	}
}
