package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

func TestOrderStatusChangeIDsDeduplicateWithoutMutatingSelection(t *testing.T) {
	input := models.OrderStatusChangeRequest{OrderIDs: []int{103, 101, 103, 102}, StatusID: 3}
	ids, err := orderStatusChangeIDs(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []int{101, 102, 103}) {
		t.Fatalf("order IDs = %v", ids)
	}
	if !reflect.DeepEqual(input.OrderIDs, []int{103, 101, 103, 102}) {
		t.Fatalf("original selection was mutated: %v", input.OrderIDs)
	}
}

func TestOrderStatusChangeIDsRejectInvalidRequests(t *testing.T) {
	for _, input := range []models.OrderStatusChangeRequest{
		{},
		{OrderIDs: []int{101}, StatusID: 0},
		{OrderIDs: []int{101}, StatusID: -1},
		{OrderIDs: []int{}, StatusID: 3},
		{OrderIDs: []int{101, 0}, StatusID: 3},
		{OrderIDs: []int{-1}, StatusID: 3},
		{OrderIDs: make([]int, orderStatusChangeMaxOrders+1), StatusID: 3},
	} {
		if _, err := orderStatusChangeIDs(input); err == nil {
			t.Fatalf("invalid input was accepted: %#v", input)
		}
	}
}

type orderStatusTestQuerier struct {
	ds.Querier
	statusMissing bool
	states        map[int]*int
	updated       []int
	synced        []int
	locks         []int
	failSync      bool
	customerUser  *int
	notifications []string
	maxMessages   []string
}

func (q *orderStatusTestQuerier) QueryRow(_ context.Context, query string, args ...any) ds.Row {
	return correctionRow(func(dest ...any) error {
		switch {
		case strings.Contains(query, "FROM public.order_statuses"):
			if q.statusMissing {
				return ds.ErrNoRows
			}
			*dest[0].(*int) = args[0].(int)
		case strings.Contains(query, "SELECT status_id, customer_user_id"):
			statusID, exists := q.states[args[0].(int)]
			if !exists {
				return ds.ErrNoRows
			}
			*dest[0].(**int) = statusID
			*dest[1].(**int) = q.customerUser
		case strings.Contains(query, "UPDATE public.orders SET status_id"):
			id := args[0].(int)
			q.updated = append(q.updated, id)
			*dest[0].(*int64) = 8
		case strings.Contains(query, "FROM public.orders AS o"):
			q.synced = append(q.synced, args[0].(int))
			if q.failSync {
				return errors.New("sync failed")
			}
			*dest[0].(*bool) = false
		case strings.Contains(query, "INSERT INTO public.max_out_messages"):
			q.maxMessages = append(q.maxMessages, args[1].(string))
			*dest[0].(*int64) = 77
			*dest[1].(*time.Time) = time.Now()
		default:
			return fmt.Errorf("unexpected query: %s", query)
		}
		return nil
	})
}

func (q *orderStatusTestQuerier) Exec(_ context.Context, query string, args ...any) (ds.ExecResult, error) {
	if strings.Contains(query, "pg_advisory_xact_lock") {
		q.locks = append(q.locks, args[1].(int))
		return productRegisterFakeExecResult{}, nil
	}
	if strings.Contains(query, "UPDATE public.order_items") {
		return productRegisterFakeExecResult{}, nil
	}
	if strings.Contains(query, "pg_notify('Order.Update'") {
		q.notifications = append(q.notifications, args[0].(string))
		return productRegisterFakeExecResult{}, nil
	}
	return nil, fmt.Errorf("unexpected exec: %s", query)
}

type orderStatusTestRows struct {
	read bool
	scan func(...any) error
}

func (r *orderStatusTestRows) Close() error { return nil }
func (r *orderStatusTestRows) Err() error   { return nil }
func (r *orderStatusTestRows) Next() bool {
	if r.read {
		return false
	}
	r.read = true
	return true
}
func (r *orderStatusTestRows) Scan(dest ...any) error { return r.scan(dest...) }

func (q *orderStatusTestQuerier) Query(_ context.Context, query string, args ...any) (ds.Rows, error) {
	switch {
	case strings.Contains(query, "FROM public.orders AS orders"):
		return &orderStatusTestRows{scan: func(dest ...any) error {
			*dest[0].(*int) = args[0].(int)
			*dest[1].(*int64) = 8
			*dest[2].(*time.Time) = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
			*dest[10].(**int) = q.customerUser
			*dest[13].(**models.Ref) = &models.Ref{Descr: "В обработке"}
			return nil
		}}, nil
	case strings.Contains(query, "FROM public.notification_templates AS t"):
		return &orderStatusTestRows{scan: func(dest ...any) error {
			*dest[0].(*int) = 1
			*dest[1].(*string) = "order_changed"
			*dest[2].(*string) = `{{if .StatusChanged}}Статус: {{.StatusName}}{{end}}`
			*dest[3].(*int64) = 999
			return nil
		}}, nil
	default:
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
}

func TestChangeOrderStatusesValidatesWholeSelectionBeforeWriting(t *testing.T) {
	oldStatus := 1
	q := &orderStatusTestQuerier{states: map[int]*int{101: &oldStatus}}
	if _, err := changeOrderStatuses(context.Background(), q, []int{101, 102}, 3); err == nil {
		t.Fatal("missing order was accepted")
	}
	if len(q.updated) != 0 || len(q.synced) != 0 {
		t.Fatal("orders were changed before the missing order was detected")
	}
}

func TestChangeOrderStatusesRejectsMissingStatusBeforeLockingOrders(t *testing.T) {
	q := &orderStatusTestQuerier{statusMissing: true}
	if _, err := changeOrderStatuses(context.Background(), q, []int{101}, 3); err == nil {
		t.Fatal("missing status was accepted")
	}
	if len(q.locks) != 0 || len(q.updated) != 0 {
		t.Fatal("orders were accessed for an invalid status")
	}
}

func TestChangeOrderStatusesSkipsUnchangedAndSynchronizesChangedOrders(t *testing.T) {
	oldStatus, targetStatus := 1, 3
	q := &orderStatusTestQuerier{states: map[int]*int{101: &oldStatus, 102: &targetStatus, 103: nil}}
	changed, err := changeOrderStatuses(context.Background(), q, []int{101, 102, 103}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 2 || !reflect.DeepEqual(q.updated, []int{101, 103}) || !reflect.DeepEqual(q.synced, q.updated) {
		t.Fatalf("changed = %d, updated = %v, synchronized = %v", changed, q.updated, q.synced)
	}
	if !reflect.DeepEqual(q.notifications, []string{`{"result":{"rows_affected":2}}`}) {
		t.Fatalf("batch event payloads = %v", q.notifications)
	}
}

func TestChangeOrderStatusesNotifiesMAXOnceAndSkipsNoOp(t *testing.T) {
	oldStatus, customerUser := 1, 7
	q := &orderStatusTestQuerier{
		states: map[int]*int{101: &oldStatus}, customerUser: &customerUser,
	}
	if _, err := changeOrderStatuses(context.Background(), q, []int{101}, 3); err != nil {
		t.Fatal(err)
	}
	if len(q.maxMessages) != 1 || !strings.Contains(q.maxMessages[0], "Статус: В обработке") {
		t.Fatalf("MAX messages = %v", q.maxMessages)
	}
	q.updated, q.synced, q.notifications, q.maxMessages = nil, nil, nil, nil
	newStatus := 3
	q.states[101] = &newStatus
	changed, err := changeOrderStatuses(context.Background(), q, []int{101}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 || len(q.updated) != 0 || len(q.synced) != 0 || len(q.notifications) != 0 || len(q.maxMessages) != 0 {
		t.Fatal("same-status batch updated the order or sent another notification")
	}
}

func TestChangeOrderStatusesPropagatesSyncFailureForTransactionRollback(t *testing.T) {
	oldStatus := 1
	q := &orderStatusTestQuerier{states: map[int]*int{101: &oldStatus, 102: &oldStatus}, failSync: true}
	if _, err := changeOrderStatuses(context.Background(), q, []int{101, 102}, 3); err == nil {
		t.Fatal("1C correction failure was ignored")
	}
	if !reflect.DeepEqual(q.updated, []int{101}) {
		t.Fatalf("batch continued after an error: %v", q.updated)
	}
}
