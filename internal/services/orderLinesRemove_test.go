package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

func TestOrderLinesRemoveIDsDeduplicateWithoutMutatingSelection(t *testing.T) {
	input := models.OrderLinesRemoveRequest{ItemIDs: []int{303, 301, 303, 302}}
	ids, err := orderLinesRemoveIDs(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []int{301, 302, 303}) {
		t.Fatalf("item IDs = %v", ids)
	}
	if !reflect.DeepEqual(input.ItemIDs, []int{303, 301, 303, 302}) {
		t.Fatalf("original selection was mutated: %v", input.ItemIDs)
	}
}

func TestOrderLinesRemoveIDsRejectInvalidRequests(t *testing.T) {
	for _, input := range []models.OrderLinesRemoveRequest{
		{},
		{ItemIDs: []int{}},
		{ItemIDs: []int{301, 0}},
		{ItemIDs: []int{-1}},
		{ItemIDs: make([]int, orderLinesRemoveMaxItems+1)},
	} {
		if _, err := orderLinesRemoveIDs(input); err == nil {
			t.Fatalf("invalid input was accepted: %#v", input)
		}
	}
}

type removeLineTestItem struct {
	orderID int
	orderItemSnapshot
}

type removeLinesTestQuerier struct {
	ds.Querier
	items             map[int]*removeLineTestItem
	versions          map[int]int64
	customerUsers     map[int]*int
	missingSnapshots  map[int]bool
	locks             []int
	parentLocks       []int
	itemWrites        []int
	versionWrites     []int
	registered        []int
	registerSnapshots map[int]map[int]orderItemSnapshot
	synced            []int
	priceClears       []int
	maxMessages       []string
	notifications     []string
	failRegister      error
	failSync          error
}

func newRemoveLinesTestQuerier() *removeLinesTestQuerier {
	return &removeLinesTestQuerier{
		items:             make(map[int]*removeLineTestItem),
		versions:          make(map[int]int64),
		customerUsers:     make(map[int]*int),
		missingSnapshots:  make(map[int]bool),
		registerSnapshots: make(map[int]map[int]orderItemSnapshot),
	}
}

func (q *removeLinesTestQuerier) addItem(id, orderID, lineNum int, required, quant float64) {
	q.items[id] = &removeLineTestItem{
		orderID: orderID,
		orderItemSnapshot: orderItemSnapshot{
			LineNum: lineNum, ProductID: id + 1000, ProductName: fmt.Sprintf("Product %d", id),
			QuantRequired: required, Quant: quant,
		},
	}
	if _, exists := q.versions[orderID]; !exists {
		q.versions[orderID] = 7
	}
}

type removeLinesTestRows struct {
	read  int
	scans []func(...any) error
}

func (r *removeLinesTestRows) Close() error { return nil }
func (r *removeLinesTestRows) Err() error   { return nil }
func (r *removeLinesTestRows) Next() bool {
	if r.read == len(r.scans) {
		return false
	}
	r.read++
	return true
}
func (r *removeLinesTestRows) Scan(dest ...any) error { return r.scans[r.read-1](dest...) }

func (q *removeLinesTestQuerier) orderedItemIDs(orderID int) []int {
	ids := make([]int, 0)
	for id, item := range q.items {
		if item.orderID == orderID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := q.items[ids[i]], q.items[ids[j]]
		if left.LineNum == right.LineNum {
			return ids[i] < ids[j]
		}
		return left.LineNum < right.LineNum
	})
	return ids
}

func (q *removeLinesTestQuerier) Query(_ context.Context, query string, args ...any) (ds.Rows, error) {
	rows := &removeLinesTestRows{}
	switch {
	case strings.Contains(query, "SELECT id, order_id FROM public.order_items"):
		for _, id := range args[0].([]int) {
			item, exists := q.items[id]
			if !exists {
				continue
			}
			rows.scans = append(rows.scans, func(dest ...any) error {
				*dest[0].(*int), *dest[1].(*int) = id, item.orderID
				return nil
			})
		}
	case strings.Contains(query, "SELECT item.id, item.line_num, item.product_id"):
		for _, id := range q.orderedItemIDs(args[0].(int)) {
			if q.missingSnapshots[id] {
				continue
			}
			item := q.items[id]
			rows.scans = append(rows.scans, func(dest ...any) error {
				*dest[0].(*int), *dest[1].(*int), *dest[2].(*int) = id, item.LineNum, item.ProductID
				*dest[3].(*string) = item.ProductName
				*dest[4].(*float64), *dest[5].(*float64) = item.QuantRequired, item.Quant
				return nil
			})
		}
	case strings.Contains(query, "FROM public.orders AS orders"):
		orderID := args[0].(int)
		for _, id := range q.orderedItemIDs(orderID) {
			item := q.items[id]
			rows.scans = append(rows.scans, func(dest ...any) error {
				*dest[0].(*int), *dest[1].(*int64) = orderID, q.versions[orderID]
				*dest[2].(*time.Time) = time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
				*dest[6].(*int), *dest[8].(*int) = 1, 2
				*dest[10].(**int) = q.customerUsers[orderID]
				*dest[13].(**models.Ref) = &models.Ref{Descr: "В обработке"}
				*dest[16].(**int), *dest[17].(**int), *dest[18].(**int) = &id, &item.LineNum, &item.ProductID
				*dest[19].(**models.Ref) = &models.Ref{Descr: item.ProductName}
				measureUnitID := 1
				*dest[20].(**int) = &measureUnitID
				*dest[22].(**float64), *dest[23].(**float64) = &item.QuantRequired, &item.Quant
				return nil
			})
		}
	case strings.Contains(query, "FROM public.notification_templates AS t"):
		rows.scans = append(rows.scans, func(dest ...any) error {
			*dest[0].(*int) = 1
			*dest[1].(*string) = "order_changed"
			*dest[2].(*string) = `{{if .StatusChanged}}STATUS CHANGED{{end}}{{.ChangedItems}}`
			*dest[3].(*int64) = 999
			return nil
		})
	default:
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	return rows, nil
}

func (q *removeLinesTestQuerier) QueryRow(_ context.Context, query string, args ...any) ds.Row {
	return correctionRow(func(dest ...any) error {
		switch {
		case strings.Contains(query, "SELECT customer_user_id FROM public.orders"):
			id := args[0].(int)
			if _, exists := q.versions[id]; !exists {
				return ds.ErrNoRows
			}
			q.parentLocks = append(q.parentLocks, id)
			*dest[0].(**int) = q.customerUsers[id]
		case strings.Contains(query, "UPDATE public.orders SET version = version + 1"):
			id := args[0].(int)
			q.versionWrites = append(q.versionWrites, id)
			q.versions[id]++
			*dest[0].(*int64) = q.versions[id]
		case strings.Contains(query, "FROM public.orders AS o"):
			q.synced = append(q.synced, args[0].(int))
			if q.failSync != nil {
				return q.failSync
			}
			*dest[0].(*bool) = false
		case strings.Contains(query, "INSERT INTO public.max_out_messages"):
			q.maxMessages = append(q.maxMessages, args[1].(string))
			*dest[0].(*int64) = int64(len(q.maxMessages))
			*dest[1].(*time.Time) = time.Now()
		default:
			return fmt.Errorf("unexpected query row: %s", query)
		}
		return nil
	})
}

type removeLinesTestExecResult int64

func (r removeLinesTestExecResult) RowsAffected() int64 { return int64(r) }

func (q *removeLinesTestQuerier) Exec(_ context.Context, query string, args ...any) (ds.ExecResult, error) {
	switch {
	case strings.Contains(query, "pg_advisory_xact_lock"):
		q.locks = append(q.locks, args[1].(int))
	case strings.Contains(query, "UPDATE public.order_items SET quant = 0"):
		if strings.Contains(query, "quant_required") {
			return nil, errors.New("line removal must not write requested quantities")
		}
		var changed int64
		for _, id := range args[1].([]int) {
			item := q.items[id]
			if item.orderID == args[0].(int) && item.Quant > 0 {
				item.Quant = 0
				q.itemWrites = append(q.itemWrites, id)
				changed++
			}
		}
		return removeLinesTestExecResult(changed), nil
	case strings.Contains(query, "public.ra_products_remove_acts"):
		if q.failRegister != nil {
			return nil, q.failRegister
		}
	case strings.Contains(query, "public.ra_products_add_act"):
		orderID := args[1].(int)
		q.registered = append(q.registered, orderID)
		q.registerSnapshots[orderID] = make(map[int]orderItemSnapshot)
		for _, id := range q.orderedItemIDs(orderID) {
			q.registerSnapshots[orderID][id] = q.items[id].orderItemSnapshot
		}
	case strings.Contains(query, "UPDATE public.order_items") && strings.Contains(query, "SET price = NULL"):
		q.priceClears = append(q.priceClears, args[0].(int))
	case strings.Contains(query, "pg_notify('Order.Update'"):
		q.notifications = append(q.notifications, args[0].(string))
	default:
		return nil, fmt.Errorf("unexpected exec: %s", query)
	}
	return removeLinesTestExecResult(0), nil
}

func assertRemoveLinesNoWrites(t *testing.T, q *removeLinesTestQuerier) {
	t.Helper()
	if len(q.itemWrites)+len(q.versionWrites)+len(q.registered)+len(q.synced)+
		len(q.priceClears)+len(q.maxMessages)+len(q.notifications) != 0 {
		t.Fatalf("unexpected writes: items=%v versions=%v register=%v sync=%v prices=%v MAX=%v events=%v",
			q.itemWrites, q.versionWrites, q.registered, q.synced, q.priceClears, q.maxMessages, q.notifications)
	}
}

func TestRemoveOrderLinesValidatesUnknownItemsBeforeLockingOrWriting(t *testing.T) {
	q := newRemoveLinesTestQuerier()
	q.addItem(301, 101, 1, 5, 5)
	if _, err := removeOrderLines(context.Background(), q, []int{301, 999}); err == nil {
		t.Fatal("unknown item was accepted")
	}
	assertRemoveLinesNoWrites(t, q)
	if len(q.locks)+len(q.parentLocks) != 0 || q.items[301].Quant != 5 {
		t.Fatal("unknown selection locked or changed an existing order")
	}
}

func TestRemoveOrderLinesRevalidatesAllSelectedSnapshotsBeforeWriting(t *testing.T) {
	q := newRemoveLinesTestQuerier()
	q.addItem(301, 101, 1, 5, 5)
	q.addItem(302, 102, 1, 3, 3)
	q.missingSnapshots[302] = true
	if _, err := removeOrderLines(context.Background(), q, []int{301, 302}); err == nil {
		t.Fatal("item replaced while acquiring locks was accepted")
	}
	assertRemoveLinesNoWrites(t, q)
	if q.items[301].Quant != 5 || q.items[302].Quant != 3 {
		t.Fatal("an earlier order was changed before revalidating the entire selection")
	}
}

func TestRemoveOrderLinesChangesOnlySelectedConfirmedQuantitiesAcrossOrders(t *testing.T) {
	q := newRemoveLinesTestQuerier()
	q.addItem(301, 102, 1, 5, 4)
	q.addItem(302, 101, 1, 3, 3)
	q.addItem(303, 101, 2, 2, 2)
	q.addItem(304, 101, 3, 9, 0)
	q.addItem(305, 101, 4, 7, 6)
	changed, err := removeOrderLines(context.Background(), q, []int{301, 302, 303, 304})
	if err != nil {
		t.Fatal(err)
	}
	if changed != 3 || !reflect.DeepEqual(q.itemWrites, []int{302, 303, 301}) {
		t.Fatalf("changed = %d, item writes = %v", changed, q.itemWrites)
	}
	for _, actual := range [][]int{q.locks, q.parentLocks, q.versionWrites, q.registered, q.synced, q.priceClears} {
		if !reflect.DeepEqual(actual, []int{101, 102}) {
			t.Fatalf("orders were not locked/updated/synchronized once in stable order: %v", actual)
		}
	}
	if len(q.items) != 5 || len(q.versions) != 2 || q.versions[101] != 8 || q.versions[102] != 8 {
		t.Fatalf("order/item identities or version increments changed: versions=%v items=%v", q.versions, q.items)
	}
	for id, required := range map[int]float64{301: 5, 302: 3, 303: 2, 304: 9, 305: 7} {
		wantQuant := float64(0)
		if id == 305 {
			wantQuant = 6
		}
		item := q.items[id]
		posted := q.registerSnapshots[item.orderID][id]
		if item.QuantRequired != required || item.Quant != wantQuant || posted.QuantRequired != required || posted.Quant != wantQuant {
			t.Fatalf("item %d has incorrect quantities: item=%#v register=%#v", id, item, posted)
		}
	}
	if !reflect.DeepEqual(q.notifications, []string{`{"result":{"rows_affected":3}}`}) {
		t.Fatalf("websocket events = %v", q.notifications)
	}
}

func TestRemoveOrderLinesNotifiesCustomerOncePerOrderAndSkipsRepeatedZeros(t *testing.T) {
	q := newRemoveLinesTestQuerier()
	userID := 7
	q.customerUsers[101], q.customerUsers[102] = &userID, &userID
	q.addItem(301, 101, 1, 5, 4)
	q.addItem(302, 101, 2, 3, 3)
	q.addItem(303, 102, 1, 2, 2)
	q.addItem(304, 101, 3, 9, 0)
	q.addItem(305, 101, 4, 7, 6)
	ids := []int{301, 302, 303, 304}
	if _, err := removeOrderLines(context.Background(), q, ids); err != nil {
		t.Fatal(err)
	}
	if len(q.maxMessages) != 2 || !strings.Contains(q.maxMessages[0], "Product 301: 5 → 0") ||
		!strings.Contains(q.maxMessages[0], "Product 302: 3 → 0") || !strings.Contains(q.maxMessages[1], "Product 303: 2 → 0") {
		t.Fatalf("MAX messages = %v", q.maxMessages)
	}
	for _, message := range q.maxMessages {
		if strings.Contains(message, "STATUS CHANGED") || strings.Contains(message, "позиция удалена") ||
			strings.Contains(message, "Product 304") || strings.Contains(message, "Product 305") {
			t.Fatalf("MAX message reports an unrelated or physical removal: %s", message)
		}
	}
	q.itemWrites, q.versionWrites, q.registered, q.synced = nil, nil, nil, nil
	q.priceClears, q.maxMessages, q.notifications = nil, nil, nil
	changed, err := removeOrderLines(context.Background(), q, ids)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 || q.versions[101] != 8 || q.versions[102] != 8 {
		t.Fatalf("zero-quantity repeat changed orders: changed=%d versions=%v", changed, q.versions)
	}
	assertRemoveLinesNoWrites(t, q)
}

func TestRemoveOrderLinesPropagatesFailuresForTransactionRollback(t *testing.T) {
	for _, failure := range []string{"register", "sync"} {
		t.Run(failure, func(t *testing.T) {
			q := newRemoveLinesTestQuerier()
			q.addItem(301, 101, 1, 5, 5)
			q.addItem(302, 102, 1, 3, 3)
			cause := errors.New(failure + " failed")
			if failure == "register" {
				q.failRegister = cause
			} else {
				q.failSync = cause
			}
			if _, err := removeOrderLines(context.Background(), q, []int{301, 302}); !errors.Is(err, cause) {
				t.Fatalf("error = %v, want %v", err, cause)
			}
			if !reflect.DeepEqual(q.itemWrites, []int{301}) || q.items[302].Quant != 3 ||
				len(q.notifications)+len(q.maxMessages) != 0 {
				t.Fatal("batch continued after failure or emitted a success notification")
			}
		})
	}
}
