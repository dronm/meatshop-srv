package integration1cworker

import (
	"net/http"
	"testing"
	"time"
)

func TestShouldRetryHTTPStatus(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusRequestTimeout, true},
		{http.StatusTooEarly, true},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}

	for _, test := range tests {
		if got := shouldRetryHTTPStatus(test.status); got != test.want {
			t.Fatalf("status %d: got %v, want %v", test.status, got, test.want)
		}
	}
}

func TestBackoff(t *testing.T) {
	base := 5 * time.Second
	maxDelay := 30 * time.Second
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}
	for index, expected := range want {
		attempt := index + 1
		if got := backoff(attempt, base, maxDelay); got != expected {
			t.Fatalf("attempt %d: got %s, want %s", attempt, got, expected)
		}
	}
}
