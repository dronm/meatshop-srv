package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func TestPrepareMaxOrderItemsPreservesUnchangedWeightsAndIdentities(t *testing.T) {
	existing := map[int]orderItemWeightState{
		11: {ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.72, WeightConfirmed: true, RequiresWeightConfirmation: true},
		12: {ID: 12, LineNum: 2, ProductID: 101, MeasureUnitID: 1, QuantRequired: 7, Quant: 6.8, WeightConfirmed: false, RequiresWeightConfirmation: true},
		13: {ID: 13, LineNum: 3, ProductID: 202, MeasureUnitID: 2, QuantRequired: 2, Quant: 0, WeightConfirmed: true},
		14: {ID: 14, LineNum: 4, ProductID: 303, MeasureUnitID: 3, QuantRequired: 3, Quant: 2.9, WeightConfirmed: true, RequiresWeightConfirmation: true},
		15: {ID: 15, LineNum: 5, ProductID: 404, MeasureUnitID: 4, QuantRequired: 1, Quant: 1.01, WeightConfirmed: true, RequiresWeightConfirmation: true},
	}
	before := make(map[int]orderItemWeightState, len(existing))
	for id, state := range existing {
		before[id] = state
	}
	products := map[int]maxOrderProductState{
		101: {MeasureUnitID: 1, RequiresWeightConfirmation: true},
		202: {MeasureUnitID: 2},
		303: {MeasureUnitID: 3, RequiresWeightConfirmation: true},
		505: {MeasureUnitID: 5},
	}
	inputs := []models.MaxOrderItemInput{
		{ProductID: 202, QuantRequired: 2},
		{ProductID: 101, QuantRequired: 7},
		{ProductID: 101, QuantRequired: 5},
		{ProductID: 303, QuantRequired: 4},
		{ProductID: 505, QuantRequired: 1},
	}
	items, err := prepareMaxOrderItems(inputs, products, existing)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []int{13, 12, 11, 14, 0}
	wantQuants := []float64{0, 6.8, 4.72, 4, 1}
	wantConfirmed := []bool{true, false, true, false, true}
	if len(items) != len(inputs) {
		t.Fatalf("item count = %d, want %d", len(items), len(inputs))
	}
	for index, item := range items {
		if item.ID != wantIDs[index] || item.Quant != wantQuants[index] || item.LineNum != index+1 ||
			item.QuantRequired != inputs[index].QuantRequired || item.ProductID != inputs[index].ProductID ||
			item.WeightConfirmed == nil || *item.WeightConfirmed != wantConfirmed[index] {
			t.Fatalf("item %d = %#v, confirmation = %v", index, item, item.WeightConfirmed)
		}
		if item.ID == 15 {
			t.Fatal("a customer-removed occurrence was retained in the resolved list")
		}
	}
	if !reflect.DeepEqual(existing, before) {
		t.Fatal("preparing MAX items mutated the saved state")
	}
	// Date/comment-only resubmissions pass the same item values and must preserve
	// every measured value, including a line removed from preparation with zero.
	again, err := prepareMaxOrderItems(inputs[:3], products, existing)
	if err != nil {
		t.Fatal(err)
	}
	for index, item := range again {
		if item.ID != wantIDs[index] || item.Quant != wantQuants[index] ||
			item.WeightConfirmed == nil || *item.WeightConfirmed != wantConfirmed[index] {
			t.Fatalf("unchanged resubmission reset a confirmed quantity: %#v", item)
		}
	}
}

func TestPrepareMaxOrderItemsReservesExactMatchesBeforeChangedOccurrences(t *testing.T) {
	existing := map[int]orderItemWeightState{
		11: {ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.72, WeightConfirmed: true},
		12: {ID: 12, LineNum: 2, ProductID: 101, MeasureUnitID: 1, QuantRequired: 7, Quant: 6.8, WeightConfirmed: true},
	}
	inputs := []models.MaxOrderItemInput{{ProductID: 101, QuantRequired: 8}, {ProductID: 101, QuantRequired: 5}}
	items, err := prepareMaxOrderItems(inputs, map[int]maxOrderProductState{
		101: {MeasureUnitID: 1, RequiresWeightConfirmation: true},
	}, existing)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != 12 || items[0].Quant != 8 || items[0].WeightConfirmed == nil || *items[0].WeightConfirmed ||
		items[1].ID != 11 || items[1].Quant != 4.72 || items[1].WeightConfirmed == nil || !*items[1].WeightConfirmed {
		t.Fatalf("changed occurrence stole the unchanged occurrence: %#v", items)
	}
}

func TestPrepareMaxOrderItemsRejectsAmbiguousIdenticalOccurrences(t *testing.T) {
	existing := map[int]orderItemWeightState{
		11: {ID: 11, LineNum: 2, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.7, WeightConfirmed: true},
		12: {ID: 12, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 0, WeightConfirmed: false},
	}
	items, err := prepareMaxOrderItems([]models.MaxOrderItemInput{{ProductID: 101, QuantRequired: 5}}, map[int]maxOrderProductState{
		101: {MeasureUnitID: 1, RequiresWeightConfirmation: true},
	}, existing)
	if err == nil || items != nil {
		t.Fatalf("ambiguous identical occurrences were guessed: items=%#v error=%v", items, err)
	}
	if existing[11].Quant != 4.7 || existing[12].Quant != 0 {
		t.Fatal("ambiguous matching changed an existing quantity")
	}
}

func TestPrepareMaxOrderItemsRejectsAmbiguousChangedRequestedQuantity(t *testing.T) {
	existing := map[int]orderItemWeightState{
		11: {ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.7, WeightConfirmed: true},
		12: {ID: 12, LineNum: 2, ProductID: 101, MeasureUnitID: 1, QuantRequired: 7, Quant: 6.8, WeightConfirmed: true},
	}
	items, err := prepareMaxOrderItems([]models.MaxOrderItemInput{{ProductID: 101, QuantRequired: 8}}, map[int]maxOrderProductState{
		101: {MeasureUnitID: 1, RequiresWeightConfirmation: true},
	}, existing)
	if err == nil || items != nil {
		t.Fatalf("ambiguous changed occurrence was guessed: items=%#v error=%v", items, err)
	}
}

func TestPrepareMaxOrderItemsPreservesUniqueExactAdminDuplicate(t *testing.T) {
	existing := map[int]orderItemWeightState{
		11: {ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.7, WeightConfirmed: true},
		12: {ID: 12, LineNum: 2, ProductID: 101, MeasureUnitID: 1, QuantRequired: 7, Quant: 6.8, WeightConfirmed: true},
	}
	items, err := prepareMaxOrderItems([]models.MaxOrderItemInput{{ProductID: 101, QuantRequired: 7}}, map[int]maxOrderProductState{
		101: {MeasureUnitID: 1, RequiresWeightConfirmation: true},
	}, existing)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 12 || items[0].Quant != 6.8 || !*items[0].WeightConfirmed {
		t.Fatalf("unique exact occurrence was not preserved: %#v", items)
	}
}

func TestPrepareMaxOrderItemsResetsChangedProductOrCurrentMeasureUnit(t *testing.T) {
	existing := map[int]orderItemWeightState{
		11: {ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.72, WeightConfirmed: true},
		12: {ID: 12, LineNum: 2, ProductID: 202, MeasureUnitID: 2, QuantRequired: 3, Quant: 2.8, WeightConfirmed: true},
	}
	items, err := prepareMaxOrderItems([]models.MaxOrderItemInput{
		{ProductID: 101, QuantRequired: 5}, {ProductID: 303, QuantRequired: 3},
	}, map[int]maxOrderProductState{
		101: {MeasureUnitID: 10, RequiresWeightConfirmation: true},
		303: {MeasureUnitID: 2},
	}, existing)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != 0 || items[0].MeasureUnitID != 10 || items[0].Quant != 5 || *items[0].WeightConfirmed ||
		items[1].ID != 0 || items[1].Quant != 3 || !*items[1].WeightConfirmed {
		t.Fatalf("changed product/unit retained an old measured state: %#v", items)
	}
}

func TestPrepareMaxOrderItemsInitializesNewWeightedAndOrdinaryLines(t *testing.T) {
	items, err := prepareMaxOrderItems([]models.MaxOrderItemInput{
		{ProductID: 101, QuantRequired: 2}, {ProductID: 202, QuantRequired: 3},
	}, map[int]maxOrderProductState{
		101: {MeasureUnitID: 1, RequiresWeightConfirmation: true},
		202: {MeasureUnitID: 2},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != 0 || items[0].Quant != 2 || *items[0].WeightConfirmed ||
		items[1].ID != 0 || items[1].Quant != 3 || !*items[1].WeightConfirmed {
		t.Fatalf("incorrect initial MAX weights: %#v", items)
	}
}

func TestPrepareMaxOrderItemsRejectsMissingProductState(t *testing.T) {
	if _, err := prepareMaxOrderItems([]models.MaxOrderItemInput{{ProductID: 101, QuantRequired: 2}}, nil, nil); err == nil {
		t.Fatal("a product without current availability/unit information was accepted")
	}
}

func TestPrepareMaxOrderItemsTreatsOrdinaryLinesAsConfirmed(t *testing.T) {
	items, err := prepareMaxOrderItems([]models.MaxOrderItemInput{{ProductID: 101, QuantRequired: 5}},
		map[int]maxOrderProductState{101: {MeasureUnitID: 1}}, map[int]orderItemWeightState{
			11: {ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 0, WeightConfirmed: false},
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 11 || items[0].Quant != 0 ||
		items[0].WeightConfirmed == nil || !*items[0].WeightConfirmed {
		t.Fatalf("ordinary unchanged line was not confirmed while retaining its zero quantity/ID: %#v", items)
	}
}

type maxOrderItemsGuardQuerier struct {
	ds.Querier
	existing     []orderItemWeightState
	productReads int
	execCalls    int
}

func (q *maxOrderItemsGuardQuerier) Query(_ context.Context, _ string, _ ...any) (ds.Rows, error) {
	rows := &removeLinesTestRows{}
	for _, item := range q.existing {
		rows.scans = append(rows.scans, func(dest ...any) error {
			*dest[0].(*int), *dest[1].(*int), *dest[2].(*int), *dest[3].(*int) =
				item.ID, item.LineNum, item.ProductID, item.MeasureUnitID
			*dest[4].(*float64), *dest[5].(*float64) = item.QuantRequired, item.Quant
			*dest[6].(*bool), *dest[7].(*bool) = item.WeightConfirmed, item.RequiresWeightConfirmation
			return nil
		})
	}
	return rows, nil
}

func (q *maxOrderItemsGuardQuerier) QueryRow(_ context.Context, _ string, _ ...any) ds.Row {
	q.productReads++
	return correctionRow(func(dest ...any) error {
		*dest[0].(*int), *dest[1].(*bool) = 1, true
		return nil
	})
}

func (q *maxOrderItemsGuardQuerier) Exec(_ context.Context, _ string, _ ...any) (ds.ExecResult, error) {
	q.execCalls++
	return nil, fmt.Errorf("unexpected write before ambiguous MAX item validation")
}

func TestSyncMaxOrderItemsRejectsAmbiguityBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name      string
		secondQty float64
		inputQty  float64
	}{
		{name: "identical exact candidates", secondQty: 5, inputQty: 5},
		{name: "changed quantity has multiple candidates", secondQty: 7, inputQty: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := &maxOrderItemsGuardQuerier{existing: []orderItemWeightState{
				{ID: 11, LineNum: 1, ProductID: 101, MeasureUnitID: 1, QuantRequired: 5, Quant: 4.7, WeightConfirmed: true, RequiresWeightConfirmation: true},
				{ID: 12, LineNum: 2, ProductID: 101, MeasureUnitID: 1, QuantRequired: test.secondQty, Quant: 6.8, WeightConfirmed: true, RequiresWeightConfirmation: true},
			}}
			err := syncMaxOrderItems(context.Background(), q, 73, []models.MaxOrderItemInput{
				{ProductID: 101, QuantRequired: test.inputQty},
			})
			var httpErr webapp.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode() != http.StatusConflict {
				t.Fatalf("error = %v, want HTTP conflict", err)
			}
			if q.productReads != 1 || q.execCalls != 0 {
				t.Fatalf("ambiguous sync read products %d times and made %d writes", q.productReads, q.execCalls)
			}
		})
	}
}
