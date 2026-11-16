package appconstants

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp/constants"
)

type constantTestRepository struct {
	mu      sync.Mutex
	records map[int64]constants.Record
}

func (r *constantTestRepository) LoadAll(context.Context) ([]constants.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]constants.Record, 0, len(r.records))
	for _, record := range r.records {
		record.Value = append(json.RawMessage(nil), record.Value...)
		result = append(result, record)
	}
	return result, nil
}

func (r *constantTestRepository) Load(_ context.Context, ids []int64) ([]constants.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]constants.Record, 0, len(ids))
	for _, id := range ids {
		if record, ok := r.records[id]; ok {
			record.Value = append(json.RawMessage(nil), record.Value...)
			result = append(result, record)
		}
	}
	return result, nil
}

func (r *constantTestRepository) Update(_ context.Context, writes []constants.Write) ([]constants.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, write := range writes {
		if r.records[write.ID].Version != write.ExpectedVersion {
			return nil, constants.ErrConflict
		}
	}
	result := make([]constants.Record, 0, len(writes))
	for _, write := range writes {
		record := r.records[write.ID]
		record.Value = append(json.RawMessage(nil), write.Value...)
		record.Version++
		r.records[write.ID] = record
		result = append(result, record)
	}
	return result, nil
}

func cashCustomerRecord(raw string, version int64) constants.Record {
	return constants.Record{
		Metadata: constants.Metadata{
			ID: 17, Code: CodeCashCustomer, Name: "Покупатель за наличный расчёт",
			ValueType: constants.TypeJSON, Version: version,
		},
		Value: json.RawMessage(raw),
	}
}

func cashCustomerManager(t *testing.T, repository *constantTestRepository, lookup func(context.Context, int) (bool, error)) *constants.Manager {
	t.Helper()
	// These tests exercise CashCustomer; other required catalogue seeds must
	// still exist so a missing cash-customer migration remains distinguishable.
	repository.mu.Lock()
	if repository.records == nil {
		repository.records = make(map[int64]constants.Record)
	}
	repository.records[18] = constants.Record{
		Metadata: constants.Metadata{
			ID: 18, Code: CodeProductionMaxChatID, Name: "Чат MAX для производства",
			ValueType: constants.TypeString, Version: 1,
		},
		Value: json.RawMessage("null"),
	}
	repository.mu.Unlock()
	registry, err := newRegistry(lookup)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := constants.NewManager(constants.Config{
		Repository: repository, Registry: registry, ReconcileInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return manager
}

func TestCashCustomerGenericAndTypedValidation(t *testing.T) {
	ctx := context.Background()
	repository := &constantTestRepository{records: map[int64]constants.Record{17: cashCustomerRecord("null", 1)}}
	lookupCalls := 0
	manager := cashCustomerManager(t, repository, func(_ context.Context, id int) (bool, error) {
		lookupCalls++
		return id == 42, nil
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start unconfigured constant: %v", err)
	}
	accessor, err := NewConstantCashCustomerService(manager)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := accessor.GetValue(ctx); err != nil || value != nil || lookupCalls != 0 {
		t.Fatalf("unconfigured value = %#v, err = %v, lookup calls = %d", value, err, lookupCalls)
	}
	for _, raw := range []string{
		`{}`, `{"keys":{}}`, `{"keys":{"id":0}}`, `{"keys":{"id":-1}}`,
		`{"keys":{"id":41}}`, `{"keys":{"id":"42"}}`,
		`{"keys":{"id":42},"unexpected":true}`, `"42"`,
	} {
		if _, err := manager.Set(ctx, []constants.Change{{ID: 17, Value: json.RawMessage(raw)}}); !errors.Is(err, constants.ErrValidation) {
			t.Fatalf("invalid reference %s: %v", raw, err)
		}
	}
	ref := &models.Ref{Descr: "Cash customer"}
	ref.Keys.ID = 42
	if err := accessor.SetValue(ctx, ref); err != nil {
		t.Fatalf("set existing customer: %v", err)
	}
	value, err := accessor.GetValue(ctx)
	if err != nil || value == nil || *value != *ref {
		t.Fatalf("typed read = %#v, err = %v", value, err)
	}
	value.Keys.ID = 99
	again, err := accessor.GetValue(ctx)
	if err != nil || again.Keys.ID != 42 {
		t.Fatalf("returned value modified shared cache: %#v, %v", again, err)
	}
	ref.Keys.ID = 41
	if err := accessor.SetValue(ctx, ref); !errors.Is(err, constants.ErrValidation) {
		t.Fatalf("typed accessor accepted nonexistent customer: %v", err)
	}
	if err := accessor.SetValue(ctx, nil); err != nil {
		t.Fatalf("clear constant: %v", err)
	}
	if value, err := accessor.GetValue(ctx); err != nil || value != nil {
		t.Fatalf("cleared value = %#v, err = %v", value, err)
	}
}

func TestCashCustomerRequiredSeedAndDatabaseRefresh(t *testing.T) {
	ctx := context.Background()
	repository := &constantTestRepository{records: map[int64]constants.Record{}}
	manager := cashCustomerManager(t, repository, func(_ context.Context, id int) (bool, error) { return id == 42, nil })
	if err := manager.Start(ctx); !errors.Is(err, constants.ErrNotFound) {
		t.Fatalf("missing migration seed: %v", err)
	}
	repository.mu.Lock()
	repository.records[17] = cashCustomerRecord("null", 1)
	repository.mu.Unlock()
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	accessor, err := NewConstantCashCustomerService(manager)
	if err != nil {
		t.Fatal(err)
	}
	repository.mu.Lock()
	repository.records[17] = cashCustomerRecord(`{"keys":{"id":42},"descr":"Edited in another process"}`, 2)
	repository.mu.Unlock()
	if err := manager.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	value, err := accessor.GetValue(ctx)
	if err != nil || value == nil || value.Keys.ID != 42 || value.Descr != "Edited in another process" {
		t.Fatalf("external assignment not refreshed: %#v, %v", value, err)
	}
}

func TestCashCustomerLookupFailureIsNotSaved(t *testing.T) {
	ctx := context.Background()
	repository := &constantTestRepository{records: map[int64]constants.Record{17: cashCustomerRecord("null", 1)}}
	lookupErr := errors.New("customer database unavailable")
	manager := cashCustomerManager(t, repository, func(context.Context, int) (bool, error) { return false, lookupErr })
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := manager.Set(ctx, []constants.Change{{ID: 17, Value: json.RawMessage(`{"keys":{"id":42}}`)}})
	if !errors.Is(err, lookupErr) {
		t.Fatalf("customer lookup error lost: %v", err)
	}
	values, err := manager.Get(ctx, []int64{17})
	if err != nil || string(values[0].Value) != "null" || values[0].Version != 1 {
		t.Fatalf("failed lookup changed stored value: %#v, %v", values, err)
	}
}
