package services

import (
	"encoding/json"
	"testing"

	"github.com/dronm/modelbind"
	"github.com/dronm/meatshop/internal/models"
)

func TestValidateObjectHistoryInput(t *testing.T) {
	query, params, err := validateObjectHistoryInput(models.ObjectHistoryInput{
		Query: &models.ObjectHistoryQuery{
			ObjectType: " construction_sites ",
			ObjectID:   " 12 ",
		},
	})
	if err != nil {
		t.Fatalf("validateObjectHistoryInput() error = %v", err)
	}
	if query.ObjectType != "construction_sites" || query.ObjectID != "12" {
		t.Fatalf("query = %#v", query)
	}
	if params.Count != objectHistoryDefaultPageSize {
		t.Fatalf("default count = %d, want %d", params.Count, objectHistoryDefaultPageSize)
	}

	_, params, err = validateObjectHistoryInput(models.ObjectHistoryInput{
		Query: &models.ObjectHistoryQuery{
			ObjectType: "construction_sites",
			ObjectID:   "12",
		},
		Params: modelbind.CollectionParams{Count: objectHistoryMaxPageSize + 1},
	})
	if err != nil {
		t.Fatalf("validateObjectHistoryInput(max count) error = %v", err)
	}
	if params.Count != objectHistoryMaxPageSize {
		t.Fatalf("limited count = %d, want %d", params.Count, objectHistoryMaxPageSize)
	}
}

func TestValidateObjectHistoryInputRejectsInvalidValues(t *testing.T) {
	tests := []models.ObjectHistoryInput{
		{Query: nil},
		{Query: &models.ObjectHistoryQuery{ObjectType: "public.users", ObjectID: "1"}},
		{Query: &models.ObjectHistoryQuery{ObjectType: "users", ObjectID: ""}},
		{
			Query:  &models.ObjectHistoryQuery{ObjectType: "users", ObjectID: "1"},
			Params: modelbind.CollectionParams{From: -1},
		},
		{
			Query: &models.ObjectHistoryQuery{ObjectType: "users", ObjectID: "1"},
			Params: modelbind.CollectionParams{
				Sorter: []modelbind.CollectionSorter{{Field: "changed_at"}},
			},
		},
	}

	for index, input := range tests {
		if _, _, err := validateObjectHistoryInput(input); err == nil {
			t.Fatalf("case %d: validateObjectHistoryInput() error = nil, want error", index)
		}
	}
}

func TestParseObjectHistoryChanges(t *testing.T) {
	raw := []byte(`[
		{"col":"name","alias":"Наименование","old":"Старое","new":"Новое","old_descr":null,"new_descr":null},
		{"col":"construction_site_id","alias":"","old":1,"new":2,"old_descr":"Склад 1","new_descr":"Склад 2"},
		{"col":"pwd","alias":"Пароль","old":"secret","new":"new-secret"}
	]`)

	changes, err := parseObjectHistoryChanges(raw)
	if err != nil {
		t.Fatalf("parseObjectHistoryChanges() error = %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("change count = %d, want 2", len(changes))
	}
	if changes[0].Field != "Наименование" || changes[0].Old != "Старое" || changes[0].New != "Новое" {
		t.Fatalf("first change = %#v", changes[0])
	}
	if changes[1].Field != "construction_site_id" {
		t.Fatalf("fallback field = %q", changes[1].Field)
	}
	if changes[1].OldDescription == nil || *changes[1].OldDescription != "Склад 1" {
		t.Fatalf("old description = %#v", changes[1].OldDescription)
	}
	if _, ok := changes[1].Old.(json.Number); !ok {
		t.Fatalf("numeric old value type = %T, want json.Number", changes[1].Old)
	}
}
