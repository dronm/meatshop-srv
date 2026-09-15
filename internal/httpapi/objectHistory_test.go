package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestObjectHistoryBinder(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/api/object-history?object_type=construction_sites&object_id=12&from=50&count=25",
		nil,
	)

	bound, err := objectHistoryBinder()(req)
	if err != nil {
		t.Fatalf("objectHistoryBinder() error = %v", err)
	}
	input, ok := bound.(models.ObjectHistoryInput)
	if !ok {
		t.Fatalf("binder result type = %T", bound)
	}
	if input.Query == nil {
		t.Fatal("binder result query is nil")
	}
	if input.Query.ObjectType != "construction_sites" || input.Query.ObjectID != "12" {
		t.Fatalf("query = %#v", input.Query)
	}
	if input.Params.From != 50 || input.Params.Count != 25 {
		t.Fatalf("collection params = %#v, want from 50 count 25", input.Params)
	}
}

func TestObjectHistoryBinderRequiresObjectKey(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/api/object-history?object_type=construction_sites",
		nil,
	)

	if _, err := objectHistoryBinder()(req); err == nil {
		t.Fatal("objectHistoryBinder() error = nil, want required object_id error")
	}
}
