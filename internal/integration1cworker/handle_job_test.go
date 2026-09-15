package integration1cworker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

type fakeExecutor struct {
	response Response
	err      error
	command  string
	params   json.RawMessage
}

func (f *fakeExecutor) Execute(ctx context.Context, command string, params json.RawMessage) (Response, error) {
	f.command = command
	f.params = append(json.RawMessage(nil), params...)
	return f.response, f.err
}

type finishCall struct {
	job       *Job
	outcome   AttemptOutcome
	retryAt   *time.Time
	lastError string
}

type fakeJobStore struct {
	finish *finishCall
}

func (f *fakeJobStore) ClaimNextJob(ctx context.Context, workerID string) (*Job, error) {
	return nil, nil
}

func (f *fakeJobStore) FinishAttempt(
	ctx context.Context,
	job *Job,
	outcome AttemptOutcome,
	retryAt *time.Time,
	lastError string,
) error {
	f.finish = &finishCall{job: job, outcome: outcome, retryAt: retryAt, lastError: lastError}
	return nil
}

func (f *fakeJobStore) RecoverStaleJobs(ctx context.Context, lockTimeout time.Duration, limit int) (int, error) {
	return 0, nil
}

func TestHandleJobKeepsHTTP400AsTerminalRawResult(t *testing.T) {
	raw := []byte(`{"success":false,"error":"already exists"}`)
	executor := &fakeExecutor{response: Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       raw,
	}}
	jobStore := &fakeJobStore{}
	service := newTestWorkerService(jobStore, executor)
	job := &Job{
		ID:           10,
		Command:      "create_order",
		Params:       json.RawMessage(`{"customer_id":"c1"}`),
		AttemptCount: 1,
		MaxAttempts:  5,
	}

	if err := service.handleJob(context.Background(), job); err != nil {
		t.Fatalf("handleJob(): %v", err)
	}
	if executor.command != job.Command || string(executor.params) != string(job.Params) {
		t.Fatalf("request changed: command=%q params=%s", executor.command, executor.params)
	}
	if jobStore.finish == nil || jobStore.finish.retryAt != nil {
		t.Fatal("HTTP 400 must be a terminal attempt")
	}
	if jobStore.finish.outcome.HTTPStatus == nil || *jobStore.finish.outcome.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("unexpected status: %#v", jobStore.finish.outcome.HTTPStatus)
	}
	if string(jobStore.finish.outcome.Body) != string(raw) {
		t.Fatalf("response body changed: %s", jobStore.finish.outcome.Body)
	}
}

func TestHandleJobRetriesHTTP503(t *testing.T) {
	executor := &fakeExecutor{response: Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Content-Type": {"text/plain"}},
		Body:       []byte("temporarily unavailable"),
	}}
	jobStore := &fakeJobStore{}
	service := newTestWorkerService(jobStore, executor)
	job := &Job{ID: 11, Command: "create_order", Params: json.RawMessage(`null`), AttemptCount: 1, MaxAttempts: 3}

	if err := service.handleJob(context.Background(), job); err != nil {
		t.Fatalf("handleJob(): %v", err)
	}
	if jobStore.finish == nil || jobStore.finish.retryAt == nil {
		t.Fatal("HTTP 503 should schedule a retry")
	}
	if jobStore.finish.lastError != "HTTP 503" {
		t.Fatalf("lastError = %q", jobStore.finish.lastError)
	}
}

func TestHandleJobStopsRetryingAfterMaxAttempts(t *testing.T) {
	executor := &fakeExecutor{err: errors.New("connection reset")}
	jobStore := &fakeJobStore{}
	service := newTestWorkerService(jobStore, executor)
	job := &Job{ID: 12, Command: "create_order", Params: json.RawMessage(`null`), AttemptCount: 3, MaxAttempts: 3}

	if err := service.handleJob(context.Background(), job); err != nil {
		t.Fatalf("handleJob(): %v", err)
	}
	if jobStore.finish == nil || jobStore.finish.retryAt != nil {
		t.Fatal("max attempts exhausted; retry must be nil")
	}
	if jobStore.finish.outcome.TransportError == "" {
		t.Fatal("transport error was not persisted")
	}
}

func newTestWorkerService(jobStore JobStore, executor Executor) *WorkerService {
	return NewWorkerService(
		jobStore,
		executor,
		"test",
		1,
		time.Second,
		time.Minute,
		time.Minute,
		time.Second,
		5*time.Second,
		time.Minute,
		make(chan struct{}),
	)
}
