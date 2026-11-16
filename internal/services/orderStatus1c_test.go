package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
)

type statusChangePreservesValues struct {
	*correctionQuery
}

func (q statusChangePreservesValues) Exec(ctx context.Context, query string, args ...any) (ds.ExecResult, error) {
	if strings.Contains(query, "UPDATE public.order_items") {
		return nil, fmt.Errorf("status-only change invalidated item prices or marking: %s", query)
	}
	return q.correctionQuery.Exec(ctx, query, args...)
}

func TestOrderStatus1CSyncKeepsCalculatedValuesAndVersionDependencies(t *testing.T) {
	for _, requested := range []bool{false, true} {
		base := &correctionQuery{requested: requested}
		q := statusChangePreservesValues{correctionQuery: base}
		if err := syncOrder1CAfterStatusChange(context.Background(), q, 101, 9); err != nil {
			t.Fatal(err)
		}
		if base.queued != requested {
			t.Fatalf("requested=%t queued=%t", requested, base.queued)
		}
		if requested {
			var metadata struct {
				Version      int64   `json:"order_version"`
				Dependencies []int64 `json:"depends_on_job_ids"`
			}
			if err := json.Unmarshal(base.metadata, &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata.Version != 9 || len(metadata.Dependencies) != 1 || metadata.Dependencies[0] != 77 {
				t.Fatalf("status-only sync lost its current version/batch dependencies: %s", base.metadata)
			}
		}
	}
}
