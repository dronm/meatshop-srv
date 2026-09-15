package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSONValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "boolean", value: `true`},
		{name: "number", value: `2`},
		{name: "string", value: `"original"`},
		{name: "object", value: `{"count":2}`},
		{name: "array", value: `[1,2]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value JSONValue
			if err := json.Unmarshal([]byte(test.value), &value); err != nil {
				t.Fatalf("unmarshal JSON value: %v", err)
			}

			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal JSON value: %v", err)
			}

			var expected any
			if err := json.Unmarshal([]byte(test.value), &expected); err != nil {
				t.Fatalf("unmarshal expected JSON: %v", err)
			}
			var actual any
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatalf("unmarshal actual JSON: %v", err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("got %#v, want %#v", actual, expected)
			}

			var scanned JSONValue
			if err := scanned.Scan(test.value); err != nil {
				t.Fatalf("scan JSON value: %v", err)
			}
			if string(scanned) != test.value {
				t.Fatalf("scanned %q, want %q", scanned, test.value)
			}
		})
	}
}

func TestJSONValueRejectsInvalidJSON(t *testing.T) {
	var value JSONValue
	if err := value.Scan("not-json"); err == nil {
		t.Fatal("invalid scanned JSON must return an error")
	}
	if err := json.Unmarshal([]byte("not-json"), &value); err == nil {
		t.Fatal("invalid request JSON must return an error")
	}
}
