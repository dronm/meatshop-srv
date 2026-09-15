package maxbot

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	base := 5 * time.Second
	max := 30 * time.Second
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 5 * time.Second},
		{attempt: 2, want: 10 * time.Second},
		{attempt: 3, want: 20 * time.Second},
		{attempt: 4, want: 30 * time.Second},
	}
	for _, tc := range cases {
		if got := retryDelay(tc.attempt, base, max); got != tc.want {
			t.Fatalf("retryDelay(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestIsRetryableSendError(t *testing.T) {
	if !isRetryableSendError(errors.New("network unavailable")) {
		t.Fatal("network errors should be retryable")
	}
	if !isRetryableSendError(&APIError{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("429 should be retryable")
	}
	if !isRetryableSendError(&APIError{StatusCode: http.StatusServiceUnavailable}) {
		t.Fatal("503 should be retryable")
	}
	if isRetryableSendError(&APIError{StatusCode: http.StatusBadRequest}) {
		t.Fatal("400 should not be retryable")
	}
}
