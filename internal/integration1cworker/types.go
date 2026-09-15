package integration1cworker

import (
	"encoding/json"
	"time"
)

type Job struct {
	ID            int64
	Command       string
	Params        json.RawMessage
	CorrelationID *string
	Metadata      json.RawMessage
	Priority      int
	AttemptCount  int
	MaxAttempts   int
	CreatedAt     time.Time
	StartedAt     *time.Time
	AttemptID     int64
	WorkerID      string
}

type AttemptOutcome struct {
	HTTPStatus     *int
	ContentType    string
	Headers        map[string][]string
	Body           []byte
	TransportError string
}

func (o AttemptOutcome) IsHTTPResponse() bool {
	return o.HTTPStatus != nil
}
