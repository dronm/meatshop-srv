package services

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

type weightItemWrite struct {
	id        int
	lineNum   int
	productID int
	unitID    int
	requested float64
	quant     float64
	confirmed bool
}

type weightPersistenceQuery struct {
	ds.Querier
	states   map[int]orderItemWeightState
	requires map[int]bool
	writes   []weightItemWrite
	removed  []int
	offsets  int
}

func (q *weightPersistenceQuery) Query(_ context.Context, query string, args ...any) (ds.Rows, error) {
	rows := &removeLinesTestRows{}
	switch {
	case strings.Contains(query, "SELECT item.id, item.line_num, item.product_id, item.measure_unit_id"):
		ids := make([]int, 0, len(q.states))
		for id := range q.states {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			state := q.states[id]
			rows.scans = append(rows.scans, func(dest ...any) error {
				*dest[0].(*int), *dest[1].(*int), *dest[2].(*int), *dest[3].(*int) = state.ID, state.LineNum, state.ProductID, state.MeasureUnitID
				*dest[4].(*float64), *dest[5].(*float64) = state.QuantRequired, state.Quant
				*dest[6].(*bool), *dest[7].(*bool) = state.WeightConfirmed, q.requires[state.ProductID]
				return nil
			})
		}
	case strings.Contains(query, "SELECT id, requires_weight_confirmation FROM public.products"):
		for _, id := range args[0].([]int) {
			requires, exists := q.requires[id]
			if !exists {
				continue
			}
			rows.scans = append(rows.scans, func(dest ...any) error {
				*dest[0].(*int), *dest[1].(*bool) = id, requires
				return nil
			})
		}
	default:
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	return rows, nil
}

func (q *weightPersistenceQuery) QueryRow(_ context.Context, query string, args ...any) ds.Row {
	return correctionRow(func(dest ...any) error {
		if !strings.Contains(query, "INSERT INTO public.order_items") || !strings.Contains(query, "weight_confirmed") {
			return fmt.Errorf("unexpected insert: %s", query)
		}
		id := 100 + len(q.writes)
		q.writes = append(q.writes, weightItemWrite{
			id: id, lineNum: args[0].(int), productID: args[2].(int), unitID: args[3].(int),
			requested: args[4].(float64), quant: args[5].(float64), confirmed: args[6].(bool),
		})
		*dest[0].(*int) = id
		return nil
	})
}

func (q *weightPersistenceQuery) Exec(_ context.Context, query string, args ...any) (ds.ExecResult, error) {
	switch {
	case strings.Contains(query, "WITH line_offset AS"):
		q.offsets++
	case strings.Contains(query, "UPDATE public.order_items"):
		if !strings.Contains(query, "weight_confirmed = $8") {
			return nil, fmt.Errorf("confirmation not persisted: %s", query)
		}
		q.writes = append(q.writes, weightItemWrite{
			id: args[0].(int), lineNum: args[2].(int), productID: args[3].(int), unitID: args[4].(int),
			requested: args[5].(float64), quant: args[6].(float64), confirmed: args[7].(bool),
		})
	case strings.Contains(query, "DELETE FROM public.order_items WHERE id = $1 AND order_id = $2"):
		q.removed = append(q.removed, args[0].(int))
	default:
		return nil, fmt.Errorf("unexpected exec: %s", query)
	}
	return removeLinesTestExecResult(1), nil
}

func TestSyncOrderItemsPersistsServerResolvedWeightStateAndStableIDs(t *testing.T) {
	q := &weightPersistenceQuery{
		states: map[int]orderItemWeightState{
			1: {ID: 1, ProductID: 10, MeasureUnitID: 20, QuantRequired: 5, Quant: 5, WeightConfirmed: false},
			2: {ID: 2, ProductID: 10, MeasureUnitID: 20, QuantRequired: 3, Quant: 3, WeightConfirmed: false},
			3: {ID: 3, ProductID: 10, MeasureUnitID: 20, QuantRequired: 2, Quant: 1.8, WeightConfirmed: true},
		},
		requires: map[int]bool{10: true, 11: false},
	}
	items := []*models.OrderDocumentItem{
		{ID: 2, ProductID: 10, MeasureUnitID: 20, QuantRequired: 3, Quant: 2.8},
		{ID: 1, ProductID: 10, MeasureUnitID: 20, QuantRequired: 5, Quant: 5},
		{ProductID: 10, MeasureUnitID: 20, QuantRequired: 4, Quant: 3.7},
		{ProductID: 11, MeasureUnitID: 20, QuantRequired: 2, Quant: 2},
	}
	if err := syncOrderItems(context.Background(), q, 77, items); err != nil {
		t.Fatal(err)
	}
	want := []weightItemWrite{
		{id: 2, lineNum: 1, productID: 10, unitID: 20, requested: 3, quant: 2.8, confirmed: true},
		{id: 1, lineNum: 2, productID: 10, unitID: 20, requested: 5, quant: 5, confirmed: false},
		{id: 102, lineNum: 3, productID: 10, unitID: 20, requested: 4, quant: 3.7, confirmed: false},
		{id: 103, lineNum: 4, productID: 11, unitID: 20, requested: 2, quant: 2, confirmed: true},
	}
	if !reflect.DeepEqual(q.writes, want) || !reflect.DeepEqual(q.removed, []int{3}) || q.offsets != 1 {
		t.Fatalf("writes=%#v removed=%v offsets=%d", q.writes, q.removed, q.offsets)
	}
	if items[2].ID != 102 || items[3].ID != 103 {
		t.Fatalf("new identities not returned: %d, %d", items[2].ID, items[3].ID)
	}
}

func TestSyncResolvedOrderItemsValidatesWholeSelectionBeforeReordering(t *testing.T) {
	confirmed := false
	for _, invalid := range []*models.OrderDocumentItem{
		{ID: 999, WeightConfirmed: &confirmed},
		{ID: 1, WeightConfirmed: &confirmed},
		{ID: 0},
	} {
		q := &weightPersistenceQuery{}
		existing := map[int]orderItemWeightState{1: {ID: 1}}
		items := []*models.OrderDocumentItem{{ID: 1, WeightConfirmed: &confirmed}, invalid}
		if err := syncResolvedOrderItems(context.Background(), q, 77, items, existing); err == nil {
			t.Fatalf("invalid row accepted: %#v", invalid)
		}
		if q.offsets != 0 || len(q.writes)+len(q.removed) != 0 {
			t.Fatalf("invalid selection wrote before validation: %#v", q)
		}
	}
}
