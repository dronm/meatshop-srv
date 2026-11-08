package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

func TestAdminOrderItemWeightConfirmationLifecycle(t *testing.T) {
	flag := func(value bool) *bool { return &value }
	original := func(confirmed bool) *orderItemWeightState {
		return &orderItemWeightState{
			ID: 41, LineNum: 1, ProductID: 10, MeasureUnitID: 20,
			QuantRequired: 5, Quant: 5, WeightConfirmed: confirmed,
			RequiresWeightConfirmation: true,
		}
	}
	input := func() *models.OrderDocumentItem {
		return &models.OrderDocumentItem{
			ID: 41, LineNum: 1, ProductID: 10, MeasureUnitID: 20,
			QuantRequired: 5, Quant: 5,
		}
	}

	tests := []struct {
		name     string
		requires bool
		previous *orderItemWeightState
		change   func(*models.OrderDocumentItem)
		want     bool
	}{
		{name: "new normal product is confirmed", want: true},
		{name: "normal product ignores an explicit pending marker", previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.WeightConfirmed = flag(false)
		}, want: true},
		{name: "new required equal quantity is pending", requires: true},
		{name: "new required differing quantity is pending", requires: true, change: func(item *models.OrderDocumentItem) {
			item.Quant = 4.3
		}},
		{name: "copied required quantity difference does not confirm", requires: true, change: func(item *models.OrderDocumentItem) {
			item.ID = 0
			item.Quant = 4.3
			item.WeightConfirmed = flag(false)
		}},
		{name: "new required explicit confirmation is accepted", requires: true, change: func(item *models.OrderDocumentItem) {
			item.WeightConfirmed = flag(true)
		}, want: true},
		{name: "legacy omission preserves pending", requires: true, previous: original(false)},
		{name: "legacy omission preserves confirmed", requires: true, previous: original(true), want: true},
		{name: "equal quantity can be explicitly confirmed", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.WeightConfirmed = flag(true)
		}, want: true},
		{name: "unchanged quantity can be explicitly reset", requires: true, previous: original(true), change: func(item *models.OrderDocumentItem) {
			item.WeightConfirmed = flag(false)
		}},
		{name: "actual quantity edit confirms", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.Quant = 4.3
		}, want: true},
		{name: "actual quantity edit overrides stale false marker", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.Quant = 4.3
			item.WeightConfirmed = flag(false)
		}, want: true},
		{name: "requested quantity edit resets confirmation", requires: true, previous: original(true), change: func(item *models.OrderDocumentItem) {
			item.QuantRequired = 7
		}},
		{name: "requested quantity edit can be explicitly confirmed", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.QuantRequired = 7
			item.WeightConfirmed = flag(true)
		}, want: true},
		{name: "requested and actual edits confirm", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.QuantRequired = 7
			item.Quant = 6.8
			item.WeightConfirmed = flag(false)
		}, want: true},
		{name: "product change resets confirmation", requires: true, previous: original(true), change: func(item *models.OrderDocumentItem) {
			item.ProductID = 11
		}},
		{name: "product change with actual edit stays pending", requires: true, previous: original(true), change: func(item *models.OrderDocumentItem) {
			item.ProductID = 11
			item.Quant = 4.3
		}},
		{name: "changed product can be explicitly confirmed", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.ProductID = 11
			item.Quant = 4.3
			item.WeightConfirmed = flag(true)
		}, want: true},
		{name: "measurement change resets confirmation", requires: true, previous: original(true), change: func(item *models.OrderDocumentItem) {
			item.MeasureUnitID = 21
			item.Quant = 4.3
		}},
		{name: "changed measurement can be explicitly confirmed", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.MeasureUnitID = 21
			item.WeightConfirmed = flag(true)
		}, want: true},
		{name: "row reordering preserves pending", requires: true, previous: original(false), change: func(item *models.OrderDocumentItem) {
			item.LineNum = 9
		}},
		{name: "row reordering preserves confirmation", requires: true, previous: original(true), change: func(item *models.OrderDocumentItem) {
			item.LineNum = 9
		}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := input()
			if test.previous == nil {
				item.ID = 0
			}
			if test.change != nil {
				test.change(item)
			}
			if got := adminOrderItemWeightConfirmed(item, test.previous, test.requires); got != test.want {
				t.Fatalf("confirmation = %t, want %t; input = %#v; previous = %#v", got, test.want, item, test.previous)
			}
		})
	}
}

// Non-weighing operations may update prices or status, but never the persisted
// confirmation marker. Exercise those operations with a writer that rejects it.
type noWeightConfirmationWrites struct {
	ds.Querier
}

func (q noWeightConfirmationWrites) Exec(ctx context.Context, query string, args ...any) (ds.ExecResult, error) {
	lower := strings.ToLower(query)
	if strings.Contains(lower, "update public.order_items") {
		set := strings.Index(lower, "set ")
		if set >= 0 {
			assignments := lower[set:]
			if where := strings.Index(assignments, "where "); where >= 0 {
				assignments = assignments[:where]
			}
			if strings.Contains(assignments, "weight_confirmed") {
				return nil, fmt.Errorf("non-weighing operation attempted to change weight confirmation")
			}
		}
	}
	return q.Querier.Exec(ctx, query, args...)
}

func TestOrderStatusChangesDoNotConfirmWeight(t *testing.T) {
	oldStatus := 1
	base := &orderStatusTestQuerier{states: map[int]*int{101: &oldStatus}}
	changed, err := changeOrderStatuses(context.Background(), noWeightConfirmationWrites{Querier: base}, []int{101}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("status changes = %d, want 1", changed)
	}
}

func TestOrder1CCorrectionDoesNotConfirmWeight(t *testing.T) {
	for _, alreadyRequested := range []bool{false, true} {
		base := &correctionQuery{requested: alreadyRequested}
		if err := syncOrder1CAfterCorrection(context.Background(), noWeightConfirmationWrites{Querier: base}, 101, 3); err != nil {
			t.Fatal(err)
		}
		if base.queued != alreadyRequested {
			t.Fatalf("already requested = %t, queued = %t", alreadyRequested, base.queued)
		}
	}
}

func TestWeightConfirmationOnlyDoesNotCreateQuantityNotifications(t *testing.T) {
	confirmed := true
	old := map[int]orderItemSnapshot{
		1: {QuantRequired: 5, Quant: 5},
		2: {QuantRequired: 5, Quant: 4.7},
	}
	items := []*models.OrderDocumentItem{
		{ID: 1, QuantRequired: 5, Quant: 5, WeightConfirmed: &confirmed},
		{ID: 2, QuantRequired: 5, Quant: 4.7, WeightConfirmed: &confirmed},
	}
	if changed := changedConfirmedQuantityItemIDs(old, items); len(changed) != 0 {
		t.Fatalf("confirmation-only save reported quantity changes: %v", changed)
	}
}
