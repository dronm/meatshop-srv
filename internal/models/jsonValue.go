package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONValue stores an arbitrary valid JSON value, including scalar values.
type JSONValue json.RawMessage

func (v *JSONValue) Scan(src any) error {
	if src == nil {
		*v = nil
		return nil
	}

	var data []byte
	switch value := src.(type) {
	case []byte:
		data = value
	case string:
		data = []byte(value)
	default:
		return fmt.Errorf("cannot scan JSON value from %T", src)
	}

	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON value: %q", string(data))
	}

	*v = append((*v)[:0], data...)
	return nil
}

func (v JSONValue) Value() (driver.Value, error) {
	if len(v) == 0 {
		return nil, nil
	}
	if !json.Valid([]byte(v)) {
		return nil, fmt.Errorf("invalid JSON value: %q", string(v))
	}

	return string(v), nil
}

func (v JSONValue) MarshalJSON() ([]byte, error) {
	if len(v) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid([]byte(v)) {
		return nil, fmt.Errorf("invalid JSON value: %q", string(v))
	}

	return []byte(v), nil
}

func (v *JSONValue) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON value: %q", string(data))
	}

	*v = append((*v)[:0], data...)
	return nil
}
