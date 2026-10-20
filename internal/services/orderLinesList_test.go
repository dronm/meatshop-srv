package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/pg"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestOrderLinesListReferenceFilters(t *testing.T) {
	for _, field := range []string{
		"customer->'keys'->>'id'",
		"customer_sale_place->'keys'->>'id'",
		"product->'keys'->>'id'",
	} {
		t.Run(field, func(t *testing.T) {
			// Decode the same numeric value sent by the grid search panel.
			var params modelbind.CollectionParams
			if err := json.Unmarshal([]byte(`{"filter":[{"f":{"`+field+`":{"o":"e","v":1000000}}}]}`), &params); err != nil {
				t.Fatal(err)
			}

			query := pg.NewPgSelect(&models.OrderLineList{}, &pg.PgFilters{}, &pg.PgSorters{}, pg.NewPgLimit(0, 30))
			if err := modelbind.BindCollectionSelectModel(query, orderLinesListParams(params)); err != nil {
				t.Fatal(err)
			}

			var args []any
			sql := query.SQL(&args)
			if !strings.Contains(sql, field+" = $1") || strings.Contains(sql, "ILIKE") {
				t.Fatalf("unexpected reference filter SQL: %s", sql)
			}
			if len(args) != 1 || args[0] != "1000000" {
				t.Fatalf("unexpected reference parameters: %#v", args)
			}
			if _, err := pgtype.NewMap().Encode(pgtype.TextOID, pgtype.TextFormatCode, args[0], nil); err != nil {
				t.Fatalf("reference key cannot be encoded as PostgreSQL text: %v", err)
			}
		})
	}
}

func TestOrderLinesListParamsPreservesOtherFilters(t *testing.T) {
	params := orderLinesListParams(modelbind.CollectionParams{
		From:   30,
		Count:  30,
		Sorter: []modelbind.CollectionSorter{{Field: "id", Direct: modelbind.SortParDesc}},
		Filter: []modelbind.CollectionFilter{{Fields: map[string]modelbind.CollectionFilterField{
			"id":                      {Operator: modelbind.FilterOperParEq, Value: float64(7)},
			"for_date":                {Operator: modelbind.FilterOperParGrEq, Value: "2026-10-20T00:00:00.000"},
			"customer->'keys'->>'id'": {Operator: modelbind.FilterOperParEq, Value: "42"},
			"product->'keys'->>'id'":  {Operator: modelbind.FilterOperParIs, Value: nil},
		}}},
	})
	fields := params.Filter[0].Fields
	if fields["id"].Value != float64(7) || fields["for_date"].Value != "2026-10-20T00:00:00.000" ||
		fields["customer->'keys'->>'id'"].Value != "42" || fields["product->'keys'->>'id'"].Value != nil {
		t.Fatalf("unrelated filter values changed: %#v", fields)
	}
	if params.From != 30 || params.Count != 30 || len(params.Sorter) != 1 || params.Sorter[0].Field != "id" {
		t.Fatalf("collection paging or sorting changed: %#v", params)
	}
}
