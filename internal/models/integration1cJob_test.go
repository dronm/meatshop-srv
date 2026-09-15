package models

import "testing"

func TestIntegration1CJobRelation(t *testing.T) {
	var model Integration1CJob
	if got := model.Relation(); got != "integration_1c.jobs" {
		t.Fatalf("Relation() = %q", got)
	}
	if model.CollectionAgg() == nil {
		t.Fatal("CollectionAgg() returned nil")
	}
}
