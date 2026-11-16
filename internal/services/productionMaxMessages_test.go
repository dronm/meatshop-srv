package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
)

func TestProductionMaxChatIDPreservesInt64AndRejectsInvalidValues(t *testing.T) {
	for _, test := range []struct {
		text string
		want int64
	}{
		{"-9223372036854775808", -9223372036854775808},
		{"9223372036854775807", 9223372036854775807},
		{" -123 ", -123},
	} {
		actual, err := parseProductionMaxChatID(&test.text)
		if err != nil || actual != test.want {
			t.Fatalf("parse %q = %d, %v", test.text, actual, err)
		}
	}
	for _, text := range []string{"", "0", "1.5", "1e10", "9223372036854775808", "chat-name"} {
		if _, err := parseProductionMaxChatID(&text); err == nil {
			t.Fatalf("invalid ID %q was accepted", text)
		}
	}
	if _, err := parseProductionMaxChatID(nil); err == nil {
		t.Fatal("unset ID was accepted")
	}
}

type productionMessageCall struct {
	query string
	args  []any
}

type productionMessageDB struct {
	chatID *string
	calls  []productionMessageCall
}

type productionMessageRow func(...any) error

func (row productionMessageRow) Scan(dest ...any) error { return row(dest...) }

type productionMessageResult int64

func (result productionMessageResult) RowsAffected() int64 { return int64(result) }

func (db *productionMessageDB) QueryRow(ctx context.Context, query string, args ...any) ds.Row {
	return productionMessageRow(func(dest ...any) error {
		if !strings.Contains(query, "webapp_constants") {
			return fmt.Errorf("unexpected query: %s", query)
		}
		*dest[0].(**string) = db.chatID
		return nil
	})
}

func (db *productionMessageDB) Query(ctx context.Context, query string, args ...any) (ds.Rows, error) {
	return nil, fmt.Errorf("unexpected Query call")
}

func (db *productionMessageDB) Exec(ctx context.Context, query string, args ...any) (ds.ExecResult, error) {
	db.calls = append(db.calls, productionMessageCall{query, args})
	return productionMessageResult(1), nil
}

func TestProductionMessageQueueSnapshotsDestinationAndStableKeys(t *testing.T) {
	chatID := "-9223372036854775808"
	db := &productionMessageDB{chatID: &chatID}
	if err := enqueueProductionMessages(context.Background(), db, 42, 7, []string{"Заявка", "Общее"}); err != nil {
		t.Fatal(err)
	}
	if len(db.calls) != 3 {
		t.Fatalf("operations = %d, want supersede + two inserts", len(db.calls))
	}
	for index, call := range db.calls[1:] {
		if call.args[0] != int64(-9223372036854775808) {
			t.Fatalf("chat id lost precision: %#v", call.args[0])
		}
		wantKey := fmt.Sprintf("production_transfer:42:7:%d", index+1)
		if call.args[3] != wantKey || call.args[4] != "production_transfer:42" || call.args[5] != index+1 {
			t.Fatalf("unstable sequence or dedup keys: %#v", call.args)
		}
		var metadata map[string]any
		if err := json.Unmarshal([]byte(call.args[2].(string)), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata["source"] != "production_transfer" || metadata["production_transfer_id"] != float64(42) || metadata["version"] != float64(7) || metadata["part"] != float64(index+1) {
			t.Fatalf("wrong document ownership: %#v", metadata)
		}
	}
}

func TestProductionMessageValidationLeavesQueueUntouched(t *testing.T) {
	chatID := "-7"
	for _, test := range []struct {
		chatID   *string
		messages []string
	}{
		{nil, []string{"Заявка"}},
		{&chatID, []string{"Заявка", ""}},
		{&chatID, []string{strings.Repeat("Я", 4001)}},
	} {
		db := &productionMessageDB{chatID: test.chatID}
		if err := enqueueProductionMessages(context.Background(), db, 42, 1, test.messages); err == nil {
			t.Fatal("invalid configuration/message accepted")
		}
		if len(db.calls) != 0 {
			t.Fatalf("validation touched queue: %#v", db.calls)
		}
	}
}
