package services

import (
	"context"
	"errors"
	"testing"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

func TestCanCustomerChangeOrder(t *testing.T) {
	for _, test := range []struct {
		code string
		want bool
	}{
		{code: "new", want: true},
		{code: "processing", want: true},
		{code: "accepted", want: false},
		{code: "approved", want: false},
		{code: "completed", want: false},
		{code: "cancelled", want: false},
		{code: "", want: false},
		{code: "unknown", want: false},
	} {
		t.Run(test.code, func(t *testing.T) {
			if got := canCustomerChangeOrder(test.code); got != test.want {
				t.Fatalf("canCustomerChangeOrder(%q) = %v, want %v", test.code, got, test.want)
			}
		})
	}
}

type initialOrderStatusQuerier struct {
	ds.Querier
	code string
	id   int
	err  error
}

func (q *initialOrderStatusQuerier) QueryRow(_ context.Context, _ string, args ...any) ds.Row {
	q.code = args[0].(string)
	return correctionRow(func(dest ...any) error {
		if q.err != nil {
			return q.err
		}
		*dest[0].(*int) = q.id
		return nil
	})
}

func TestDefaultAdminOrderStatusResolvesNewByCode(t *testing.T) {
	q := &initialOrderStatusQuerier{id: 42}
	document := &models.OrderDocument{}
	if err := defaultAdminOrderStatus(context.Background(), q, document); err != nil {
		t.Fatal(err)
	}
	if q.code != "new" || document.StatusID == nil || *document.StatusID != 42 {
		t.Fatalf("lookup code = %q, document status = %v", q.code, document.StatusID)
	}
}

func TestDefaultAdminOrderStatusPreservesExplicitSelection(t *testing.T) {
	for _, id := range []int{1, 2, 3, 4, 5, 6} {
		document := &models.OrderDocument{StatusID: &id}
		if err := defaultAdminOrderStatus(context.Background(), nil, document); err != nil {
			t.Fatal(err)
		}
		if document.StatusID != &id {
			t.Fatalf("explicit status %d was replaced", id)
		}
	}
}

func TestDefaultAdminOrderStatusPropagatesMissingStatus(t *testing.T) {
	q := &initialOrderStatusQuerier{err: ds.ErrNoRows}
	document := &models.OrderDocument{}
	err := defaultAdminOrderStatus(context.Background(), q, document)
	if !errors.Is(err, ds.ErrNoRows) || document.StatusID != nil {
		t.Fatalf("error = %v, document status = %v", err, document.StatusID)
	}
}
