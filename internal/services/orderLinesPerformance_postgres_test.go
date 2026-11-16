package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	wmodels "github.com/dronm/webapp/models"
)

func TestOrderLinesPostgresProjectionPreservesRowsAndUsesJoin(t *testing.T) {
	pool, provider := productionTransferPostgres(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		ALTER TABLE public.orders ADD COLUMN shipment_ref_1c jsonb, ADD COLUMN shipment_number_1c text;
		ALTER TABLE public.customers ADD COLUMN use_marking boolean NOT NULL DEFAULT false;
		ALTER TABLE public.products ADD COLUMN requires_weight_confirmation boolean NOT NULL DEFAULT false;
		ALTER TABLE public.order_items ADD COLUMN weight_confirmed boolean NOT NULL DEFAULT false,
			ADD COLUMN mercury boolean NOT NULL DEFAULT false;
		INSERT INTO public.production_transfers(id,document_date,for_date) VALUES(701,'2026-11-13','2026-11-14');
		INSERT INTO public.production_transfer_orders(transfer_id,order_id,line_num) VALUES(701,101,1);
	`); err != nil {
		t.Fatal(err)
	}
	for _, function := range []string{"customers_ref", "customer_sale_places_ref", "products_ref", "order_statuses_ref"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "sql", function+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, migration := range []string{"000088_order_lines_production_reference.up.sql", "000089_order_lines_query_performance.up.sql"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", migration, err)
		}
	}
	var count, stableReferences int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.order_lines_list`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("optional production join changed active item count: count=%d, err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
		WHERE n.nspname='public' AND p.proname=ANY($1::text[]) AND p.provolatile='s'`,
		[]string{"customers_ref", "customer_sale_places_ref", "products_ref", "order_statuses_ref"}).Scan(&stableReferences); err != nil || stableReferences != 4 {
		t.Fatalf("reference builders cannot be pruned from counts: stable=%d, err=%v", stableReferences, err)
	}
	var plan []byte
	if err := pool.QueryRow(ctx, `EXPLAIN (ANALYZE, FORMAT JSON)
		SELECT * FROM public.order_lines_list ORDER BY id DESC,line_num,item_id LIMIT 30`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(plan, &decoded); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plan), `"Subplan Name"`) {
		t.Fatalf("production navigation requires a correlated subplan: %s", plan)
	}
	service := &OrderService{DB: provider, Session: customerMessageTestSession{user: models.UserLogin{ID: 101, RoleID: models.RoleIDAdmin}}}
	params := modelbind.CollectionParams{
		Count: 2,
		Sorter: []modelbind.CollectionSorter{
			{Field: "id", Direct: modelbind.SortParDesc},
			{Field: "line_num", Direct: modelbind.SortParAsc},
			{Field: "item_id", Direct: modelbind.SortParAsc},
		},
	}
	first, err := service.LinesList(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	params.From = 2
	second, err := service.LinesList(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	totals, ok := first.Agg.(*wmodels.TotCount)
	if len(first.Rows) != 2 || len(second.Rows) != 1 || !ok || totals.TotCount != 3 {
		t.Fatalf("paging changed: first=%#v second=%#v", first, second)
	}
	seen := map[int]bool{}
	for _, row := range append(first.Rows, second.Rows...) {
		if seen[row.ItemID] {
			t.Fatalf("join duplicated item %d across pages", row.ItemID)
		}
		seen[row.ItemID] = true
		if row.ID == 101 {
			if row.ProductionTransferID == nil || *row.ProductionTransferID != 701 {
				t.Fatalf("production document reference lost: %#v", row)
			}
		} else if row.ProductionTransferID != nil {
			t.Fatalf("unlinked order acquired a production document: %#v", row)
		}
	}
	params.From = 0
	params.Filter = []modelbind.CollectionFilter{{Fields: map[string]modelbind.CollectionFilterField{
		"customer->'keys'->>'id'": {Operator: modelbind.FilterOperParEq, Value: float64(1)},
	}}}
	filtered, err := service.LinesList(ctx, params)
	if err != nil || len(filtered.Rows) != 1 || filtered.Rows[0].ID != 101 {
		t.Fatalf("reference filtering changed after query optimization: %#v, err=%v", filtered, err)
	}
}
