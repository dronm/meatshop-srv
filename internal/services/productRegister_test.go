package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
)

type productRegisterExecCall struct {
	query string
	args  []any
}

type productRegisterFakeQuerier struct {
	execCalls []productRegisterExecCall
	execErrAt int
}

func (q *productRegisterFakeQuerier) Exec(
	_ context.Context,
	query string,
	args ...any,
) (ds.ExecResult, error) {
	q.execCalls = append(q.execCalls, productRegisterExecCall{
		query: query,
		args:  append([]any(nil), args...),
	})
	if q.execErrAt > 0 && len(q.execCalls) == q.execErrAt {
		return nil, errors.New("exec failed")
	}
	return productRegisterFakeExecResult{}, nil
}

func (q *productRegisterFakeQuerier) Query(context.Context, string, ...any) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *productRegisterFakeQuerier) QueryRow(context.Context, string, ...any) ds.Row {
	return productRegisterFakeRow{}
}

type productRegisterFakeExecResult struct{}

func (productRegisterFakeExecResult) RowsAffected() int64 {
	return 0
}

type productRegisterFakeRow struct{}

func (productRegisterFakeRow) Scan(...any) error {
	return errors.New("unexpected QueryRow call")
}

func TestRebuildProductRegisterActions(t *testing.T) {
	querier := &productRegisterFakeQuerier{}
	if err := rebuildProductRegisterActions(context.Background(), querier, orderRecorderType, 17); err != nil {
		t.Fatalf("rebuildProductRegisterActions() error = %v", err)
	}

	if len(querier.execCalls) != 2 {
		t.Fatalf("Exec call count = %d, want 2", len(querier.execCalls))
	}
	if !strings.Contains(querier.execCalls[0].query, "public.ra_products_remove_acts") {
		t.Fatalf("first query does not remove old actions: %s", querier.execCalls[0].query)
	}

	writeQuery := querier.execCalls[1].query
	for _, part := range []string{
		"public.ra_products_add_act",
		"public.register_date_start(orders.for_date)",
		"orders.customer_id",
		"orders.customer_sale_place_id",
		"item.product_id",
		"item.measure_unit_id",
		"item.quant_required",
		"item.quant",
	} {
		if !strings.Contains(writeQuery, part) {
			t.Errorf("write query does not contain %q", part)
		}
	}
	if len(querier.execCalls[1].args) != 2 ||
		querier.execCalls[1].args[0] != orderRecorderType ||
		querier.execCalls[1].args[1] != 17 {
		t.Fatalf("write args = %#v", querier.execCalls[1].args)
	}
}

func TestLockProductRegisterRecordersSortsAndDeduplicates(t *testing.T) {
	querier := &productRegisterFakeQuerier{}
	if err := lockProductRegisterRecorders(
		context.Background(),
		querier,
		orderRecorderType,
		5,
		2,
		5,
		0,
		3,
	); err != nil {
		t.Fatalf("lockProductRegisterRecorders() error = %v", err)
	}

	wantIDs := []any{2, 3, 5}
	if len(querier.execCalls) != len(wantIDs) {
		t.Fatalf("Exec call count = %d, want %d", len(querier.execCalls), len(wantIDs))
	}
	for index, wantID := range wantIDs {
		call := querier.execCalls[index]
		if len(call.args) != 2 || call.args[1] != wantID {
			t.Errorf("lock call %d args = %#v, want recorder id %v", index, call.args, wantID)
		}
	}
}

func TestRebuildProductRegisterActionsRejectsUnknownRecorder(t *testing.T) {
	querier := &productRegisterFakeQuerier{}
	err := rebuildProductRegisterActions(context.Background(), querier, "Unknown", 17)
	if err == nil {
		t.Fatal("unknown recorder type was accepted")
	}
}
