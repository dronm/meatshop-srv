package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

type productionTotal struct {
	ProductID, UnitID     int
	ProductName, UnitName string
	Quant                 *big.Rat
}
type productionUnitTotals struct {
	UnitID   int
	UnitName string
	Products []*productionTotal
	Quant    *big.Rat
}

func productionTotals(orders []*models.ProductionOrderRow) ([]*productionUnitTotals, error) {
	byProduct := make(map[[2]int]*productionTotal)
	for _, order := range orders {
		for _, item := range order.Items {
			quant, ok := new(big.Rat).SetString(item.Quant)
			if !ok || quant.Sign() < 0 {
				return nil, webapp.Internal("Некорректное количество продукции в заказе.", nil)
			}
			key := [2]int{item.ProductID, item.MeasureUnitID}
			total := byProduct[key]
			if total == nil {
				total = &productionTotal{ProductID: item.ProductID, UnitID: item.MeasureUnitID, ProductName: item.ProductName, UnitName: item.MeasureUnitName, Quant: new(big.Rat)}
				byProduct[key] = total
			}
			total.Quant.Add(total.Quant, quant)
		}
	}
	byUnit := make(map[int]*productionUnitTotals)
	for _, product := range byProduct {
		unit := byUnit[product.UnitID]
		if unit == nil {
			unit = &productionUnitTotals{UnitID: product.UnitID, UnitName: product.UnitName, Quant: new(big.Rat)}
			byUnit[product.UnitID] = unit
		}
		unit.Products = append(unit.Products, product)
		unit.Quant.Add(unit.Quant, product.Quant)
	}
	result := make([]*productionUnitTotals, 0, len(byUnit))
	for _, unit := range byUnit {
		sort.Slice(unit.Products, func(i, j int) bool {
			a, b := unit.Products[i], unit.Products[j]
			if a.ProductName == b.ProductName {
				return a.ProductID < b.ProductID
			}
			return a.ProductName < b.ProductName
		})
		result = append(result, unit)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if productionUnitPriority(a.UnitName) != productionUnitPriority(b.UnitName) {
			return productionUnitPriority(a.UnitName) < productionUnitPriority(b.UnitName)
		}
		if a.UnitName == b.UnitName {
			return a.UnitID < b.UnitID
		}
		return a.UnitName < b.UnitName
	})
	return result, nil
}
func productionUnitPriority(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "кг":
		return 0
	case "кор":
		return 1
	default:
		return 2
	}
}
func productionRatString(quant *big.Rat) string {
	return strings.ReplaceAll(strings.TrimRight(strings.TrimRight(quant.FloatString(4), "0"), "."), ".", ",")
}
func productionQuantity(value string) string {
	quant, ok := new(big.Rat).SetString(value)
	if !ok {
		return value
	}
	return productionRatString(quant)
}
func productionOrderHeading(order *models.ProductionOrderRow) string {
	name := ""
	if order.Customer != nil {
		name = order.Customer.Descr
	}
	if order.CustomerSalePlace != nil && order.CustomerSalePlace.Descr != "" {
		name += " -- " + order.CustomerSalePlace.Descr
	}
	return name
}
func productionTotalBlocks(orders []*models.ProductionOrderRow) ([]string, error) {
	groups, err := productionTotals(orders)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		var b strings.Builder
		b.WriteString(group.UnitName + ":\n")
		for _, product := range group.Products {
			fmt.Fprintf(&b, "%s -- %s %s\n", product.ProductName, productionRatString(product.Quant), group.UnitName)
		}
		fmt.Fprintf(&b, "Итого: %s %s", productionRatString(group.Quant), group.UnitName)
		result = append(result, b.String())
	}
	if len(result) == 0 {
		result = append(result, "Нет продукции.")
	}
	return result, nil
}

func productionMessages(forDate time.Time, orders []*models.ProductionOrderRow, revised bool) ([]string, error) {
	title := "Заявка на " + forDate.Format("02.01.06")
	if revised {
		title += " (изменено)"
	}
	blocks := make([]string, 0, len(orders))
	for _, order := range orders {
		var b strings.Builder
		b.WriteString(productionOrderHeading(order))
		b.WriteByte('\n')
		for _, item := range order.Items {
			fmt.Fprintf(&b, "%s -- %s %s\n", item.ProductName, productionQuantity(item.Quant), item.MeasureUnitName)
		}
		b.WriteString("________________________________________")
		blocks = append(blocks, b.String())
	}
	if len(blocks) == 0 {
		blocks = append(blocks, "Все заказы удалены из передачи в производство.")
	}
	result := splitProductionMessage(title, blocks, 4000)
	totals, err := productionTotalBlocks(orders)
	if err != nil {
		return nil, err
	}
	result = append(result, splitProductionMessage("Общее:", totals, 4000)...)
	return result, nil
}

// Split on whole order/unit blocks where possible, and on lines/runes when a
// single block is itself too large. MAX limits Unicode characters, not bytes.
func splitProductionMessage(title string, blocks []string, limit int) []string {
	prefix := title + "\n\n"
	capacity := limit - utf8.RuneCountInString(prefix)
	if capacity < 1 {
		return nil
	}
	parts := make([]string, 0)
	current := ""
	flush := func() {
		if current != "" {
			parts = append(parts, prefix+current)
			current = ""
		}
	}
	for _, block := range blocks {
		if current != "" && utf8.RuneCountInString(current)+2+utf8.RuneCountInString(block) > capacity {
			flush()
		}
		if utf8.RuneCountInString(block) <= capacity {
			if current != "" {
				current += "\n\n"
			}
			current += block
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			for utf8.RuneCountInString(line) > capacity {
				flush()
				runes := []rune(line)
				parts = append(parts, prefix+string(runes[:capacity]))
				line = string(runes[capacity:])
			}
			separator := ""
			if current != "" {
				separator = "\n"
			}
			if utf8.RuneCountInString(current+separator+line) > capacity {
				flush()
				separator = ""
			}
			current += separator + line
		}
	}
	flush()
	return parts
}

// GetProductionTransferPrint is called only by the permission-protected route.
func GetProductionTransferPrint(ctx context.Context, db ds.Provider, id int) ([]byte, error) {
	if db == nil {
		return nil, webapp.Internal("База данных недоступна.", nil)
	}
	conn, key, err := db.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Release(conn, key)
	document, err := readProductionDocument(ctx, conn.Conn(), id, true)
	if err != nil {
		return nil, err
	}
	return renderProductionTransfer(document)
}

func renderProductionTransfer(document *models.ProductionTransferDocument) ([]byte, error) {
	totals, err := productionTotalBlocks(document.Orders)
	if err != nil {
		return nil, err
	}
	data := struct {
		Document *models.ProductionTransferDocument
		Title    string
		Totals   string
	}{document, "Заявка на " + document.ForDate.Format("02.01.06"), strings.Join(totals, "\n\n")}
	var b bytes.Buffer
	if err := productionPrintTemplate.Execute(&b, data); err != nil {
		return nil, fmt.Errorf("не удалось сформировать печатную форму: %w", err)
	}
	return b.Bytes(), nil
}

var productionPrintTemplate = template.Must(template.New("production").Funcs(template.FuncMap{
	"heading": productionOrderHeading, "quant": productionQuantity,
	"mark": func(value bool) string {
		if value {
			return "Да"
		}
		return "Нет"
	},
}).Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title>
<style>
@page { size: A4 landscape; margin: 12mm; }
body { font: 12px Arial, sans-serif; color: #111; }
h1 { font-size: 20px; margin: 0 0 8px; } .meta { margin: 0 0 12px; }
table { border-collapse: collapse; width: 100%; table-layout: fixed; }
th, td { border: 1px solid #777; padding: 6px; vertical-align: top; overflow-wrap: anywhere; white-space: pre-wrap; }
th { background: #eee; text-align: left; } .order { background: #f2f2f2; font-weight: bold; break-after: avoid; }
.order small { display: block; font-weight: normal; margin-top: 3px; } .mark { text-align: center; }
thead { display: table-header-group; } tr { break-inside: avoid; } pre { font: inherit; white-space: pre-wrap; }
.controls { margin-bottom: 12px; } @media print { .controls { display: none; } }
</style></head><body><div class="controls"><button onclick="window.print()">Печать</button></div>
<h1>{{.Title}}</h1><p class="meta">Передача в производство №{{.Document.ID}} от {{.Document.DocumentDate.Format "02.01.2006"}}{{with .Document.Comment}} — {{.}}{{end}}</p>
<table><colgroup><col style="width:49%"><col style="width:9%"><col style="width:21%"><col style="width:21%"></colgroup>
<thead><tr><th>Продукция и количество</th><th>Маркировка</th><th>Комментарий клиента</th><th>Комментарий менеджера</th></tr></thead>
{{range .Document.Orders}}<tbody><tr class="order"><td colspan="2">{{heading .}}{{with .Order}}<small>{{.Descr}}</small>{{end}}</td><td>{{with .CommentCustomer}}{{.}}{{end}}</td><td>{{with .CommentAdmin}}{{.}}{{end}}</td></tr>
{{range .Items}}<tr><td>{{.ProductName}} — {{quant .Quant}} {{.MeasureUnitName}}</td><td class="mark">{{mark .UseMarking}}</td><td></td><td></td></tr>{{end}}</tbody>{{end}}
</table><h2>Общее:</h2><pre>{{.Totals}}</pre></body></html>`))
