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

func TestWeightConfirmationSelectionRequiresExplicitFlagAndValidIDs(t *testing.T) {
	confirmed := false
	for _, input := range []models.OrderLinesWeightConfirmationRequest{
		{ItemIDs: []int{1}},
		{WeightConfirmed: &confirmed},
		{ItemIDs: []int{0}, WeightConfirmed: &confirmed},
		{ItemIDs: []int{-1}, WeightConfirmed: &confirmed},
		{ItemIDs: make([]int, orderLineIDsMaxItems+1), WeightConfirmed: &confirmed},
	} {
		if _, err := weightConfirmationItemIDs(input); err == nil {
			t.Fatalf("invalid input accepted: %#v", input)
		}
	}
	input := models.OrderLinesWeightConfirmationRequest{ItemIDs: []int{3, 1, 3}, WeightConfirmed: &confirmed}
	ids, err := weightConfirmationItemIDs(input)
	if err != nil || !reflect.DeepEqual(ids, []int{1, 3}) || !reflect.DeepEqual(input.ItemIDs, []int{3, 1, 3}) {
		t.Fatalf("selection validation: ids=%v, input=%v, err=%v", ids, input.ItemIDs, err)
	}
}

type weightConfirmationTestItem struct {
	orderID   int
	quant     float64
	required  bool
	confirmed bool
}

type weightConfirmationTestQuerier struct {
	ds.Querier
	items         map[int]*weightConfirmationTestItem
	parentLocks   []int
	calls         []string
	writes        []int
	notifications []string
	excludeUpdate bool
}

func (q *weightConfirmationTestQuerier) Exec(_ context.Context, query string, args ...any) (ds.ExecResult, error) {
	switch {
	case strings.Contains(query, "SELECT id FROM public.orders"):
		if !strings.Contains(query, "ORDER BY id FOR UPDATE") {
			return nil, fmt.Errorf("missing stable parent lock order")
		}
		orders := make(map[int]struct{})
		for _, id := range args[0].([]int) {
			if item := q.items[id]; item != nil {
				orders[item.orderID] = struct{}{}
			}
		}
		for id := range orders {
			q.parentLocks = append(q.parentLocks, id)
		}
		sort.Ints(q.parentLocks)
		q.calls = append(q.calls, "parents")
	case strings.Contains(query, "UPDATE public.order_items AS item SET weight_confirmed"):
		q.calls = append(q.calls, "update")
		var count int64
		if !q.excludeUpdate {
			for _, id := range args[0].([]int) {
				item := q.items[id]
				if item.quant > 0 && item.required && item.confirmed != args[1].(bool) {
					item.confirmed = args[1].(bool)
					q.writes = append(q.writes, id)
					count++
				}
			}
		}
		return removeLinesTestExecResult(count), nil
	case strings.Contains(query, "pg_notify('Order.Update'"):
		q.calls = append(q.calls, "notify")
		q.notifications = append(q.notifications, args[0].(string))
	default:
		return nil, fmt.Errorf("unexpected write or 1C operation: %s", query)
	}
	return removeLinesTestExecResult(0), nil
}

func (q *weightConfirmationTestQuerier) Query(_ context.Context, query string, args ...any) (ds.Rows, error) {
	if !strings.Contains(query, "ORDER BY item.id FOR UPDATE OF item") {
		return nil, fmt.Errorf("missing stable item lock order")
	}
	q.calls = append(q.calls, "items")
	rows := &removeLinesTestRows{}
	for _, id := range args[0].([]int) {
		item := q.items[id]
		if item == nil {
			continue
		}
		rows.scans = append(rows.scans, func(dest ...any) error {
			*dest[0].(*int), *dest[1].(*float64) = id, item.quant
			*dest[2].(*bool), *dest[3].(*bool) = item.required, item.confirmed
			return nil
		})
	}
	return rows, nil
}

func TestSetWeightConfirmationMixedSelectionAndRepeatedCommand(t *testing.T) {
	q := &weightConfirmationTestQuerier{items: map[int]*weightConfirmationTestItem{
		1: {orderID: 20, quant: 21.1, required: true},
		2: {orderID: 10, quant: 10, required: true, confirmed: true},
		3: {orderID: 10, quant: 5},
		4: {orderID: 20, required: true},
	}}
	changed, err := setOrderLinesWeightConfirmed(t.Context(), q, []int{1, 2, 3, 4}, true)
	if err != nil || changed != 1 {
		t.Fatalf("confirm mixed selection: changed=%d, err=%v", changed, err)
	}
	if !reflect.DeepEqual(q.writes, []int{1}) || !reflect.DeepEqual(q.parentLocks, []int{10, 20}) ||
		!reflect.DeepEqual(q.calls, []string{"parents", "items", "update", "notify"}) {
		t.Fatalf("unexpected confirmation flow: %#v", q)
	}
	if q.items[1].quant != 21.1 || q.items[3].confirmed || q.items[4].confirmed {
		t.Fatal("quantity, ordinary product, or removed item was changed")
	}
	changed, err = setOrderLinesWeightConfirmed(t.Context(), q, []int{1, 2, 3, 4}, true)
	if err != nil || changed != 0 || len(q.notifications) != 1 || len(q.writes) != 1 {
		t.Fatalf("repeated command was not idempotent: changed=%d, err=%v", changed, err)
	}
}

func TestResetWeightConfirmationOnlyChangesEligibleLines(t *testing.T) {
	q := &weightConfirmationTestQuerier{items: map[int]*weightConfirmationTestItem{
		1: {orderID: 10, quant: 21.1, required: true, confirmed: true},
		2: {orderID: 10, quant: 10, confirmed: true},
		3: {orderID: 20, required: true, confirmed: true},
	}}
	changed, err := setOrderLinesWeightConfirmed(t.Context(), q, []int{1, 2, 3}, false)
	if err != nil || changed != 1 || q.items[1].confirmed || !q.items[2].confirmed || !q.items[3].confirmed {
		t.Fatalf("reset mixed selection: changed=%d, err=%v, items=%#v", changed, err, q.items)
	}
	if q.items[1].quant != 21.1 || q.notifications[0] != `{"result":{"rows_affected":1}}` {
		t.Fatal("reset changed quantity or emitted an invalid refresh notification")
	}
}

func TestWeightConfirmationMissingSelectionDoesNotPartiallyWrite(t *testing.T) {
	q := &weightConfirmationTestQuerier{items: map[int]*weightConfirmationTestItem{
		1: {orderID: 10, quant: 10, required: true},
	}}
	if _, err := setOrderLinesWeightConfirmed(t.Context(), q, []int{1, 99}, true); err == nil {
		t.Fatal("missing selected line accepted")
	}
	if len(q.writes)+len(q.notifications) != 0 || q.items[1].confirmed {
		t.Fatal("a partial selection was changed")
	}
}

func TestWeightConfirmationReportsConcurrentPolicyChangeBeforeNotification(t *testing.T) {
	q := &weightConfirmationTestQuerier{excludeUpdate: true, items: map[int]*weightConfirmationTestItem{
		1: {orderID: 10, quant: 10, required: true},
	}}
	if _, err := setOrderLinesWeightConfirmed(t.Context(), q, []int{1}, true); err == nil {
		t.Fatal("concurrent policy change accepted")
	}
	if len(q.notifications) != 0 {
		t.Fatal("failed update emitted a notification")
	}
}
