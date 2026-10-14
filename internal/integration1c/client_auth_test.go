package integration1c

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientOptionalBasicAuth(t *testing.T) {
	endpoints := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "execute",
			path: "/execute",
			call: func(client *Client) error {
				_, err := client.CompleteNomenclature(context.Background(), "meat")
				return err
			},
		},
		{
			name: "binary",
			path: "/bin-data",
			call: func(client *Client) error {
				_, err := client.PrintOrder(context.Background(), []int{1})
				return err
			},
		},
	}

	credentials := []struct {
		name string
		auth *Auth
	}{
		{name: "absent"},
		{name: "configured", auth: &Auth{UserName: "integration-user", Password: "p@ss:word"}},
		{name: "empty password", auth: &Auth{UserName: "integration-user"}},
	}

	for _, endpoint := range endpoints {
		for _, credential := range credentials {
			t.Run(endpoint.name+"/"+credential.name, func(t *testing.T) {
				var requests atomic.Int32
				var wantAuthorization string
				if credential.auth != nil {
					wantAuthorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(
						credential.auth.UserName+":"+credential.auth.Password,
					))
				}

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestNo := requests.Add(1)
					if r.URL.Path != endpoint.path {
						t.Errorf("path = %q, want %q", r.URL.Path, endpoint.path)
					}
					if got := r.Header.Get("Authorization"); got != wantAuthorization {
						t.Errorf("request %d Authorization = %q, want %q", requestNo, got, wantAuthorization)
					}
					if credential.auth != nil {
						userName, password, ok := r.BasicAuth()
						if !ok || userName != credential.auth.UserName || password != credential.auth.Password {
							t.Errorf("request %d credentials = (%q, %q, %t)", requestNo, userName, password, ok)
						}
					}
					if requestNo == 1 {
						http.Error(w, "temporary", http.StatusServiceUnavailable)
						return
					}
					if endpoint.path == "/bin-data" {
						w.Header().Set("Content-Type", "application/pdf")
						_, _ = w.Write([]byte("%PDF-1.7\nmock"))
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"success":true,"payload":[]}`))
				}))
				defer server.Close()

				client, err := NewClient(Config{
					URL:        server.URL,
					MaxRetries: 1,
					RetryDelay: time.Millisecond,
					Auth:       credential.auth,
				})
				if err != nil {
					t.Fatalf("NewClient(): %v", err)
				}
				if err := endpoint.call(client); err != nil {
					t.Fatalf("request failed: %v", err)
				}
				if got := requests.Load(); got != 2 {
					t.Fatalf("requests = %d, want 2", got)
				}
			})
		}
	}
}
