package integration1cworker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayExecutePreservesCommandAndRawResponse(t *testing.T) {
	responseBody := []byte(`{"success":true,"payload":{"id":"doc-1","descr":"Order 1"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/execute" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var request struct {
			Command string          `json:"command"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if request.Command != "create_order" {
			t.Fatalf("command = %q", request.Command)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	client, err := NewGatewayClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewGatewayClient(): %v", err)
	}
	response, err := client.Execute(context.Background(), "create_order", json.RawMessage(`{"customer_id":"c1"}`))
	if err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if response.StatusCode != http.StatusOK || string(response.Body) != string(responseBody) {
		t.Fatalf("unexpected response: %#v %s", response, response.Body)
	}
}

func TestGatewayExecuteOptionalBasicAuth(t *testing.T) {
	tests := []struct {
		name              string
		auth              *Auth
		wantAuthorization string
	}{
		{
			name: "omitted",
		},
		{
			name:              "configured",
			auth:              &Auth{UserName: "gateway-user", Password: "secret"},
			wantAuthorization: "Basic Z2F0ZXdheS11c2VyOnNlY3JldA==",
		},
		{
			name:              "blank password",
			auth:              &Auth{UserName: "gateway-user"},
			wantAuthorization: "Basic Z2F0ZXdheS11c2VyOg==",
		},
		{
			name:              "unicode and colon password",
			auth:              &Auth{UserName: "пользователь", Password: "пароль:with:colon"},
			wantAuthorization: "Basic 0L/QvtC70YzQt9C+0LLQsNGC0LXQu9GMOtC/0LDRgNC+0LvRjDp3aXRoOmNvbG9u",
		},
		{
			name:              "present empty auth",
			auth:              &Auth{},
			wantAuthorization: "Basic Og==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorization := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authorization <- r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client, err := NewGatewayClient(server.URL, tt.auth)
			if err != nil {
				t.Fatalf("NewGatewayClient(): %v", err)
			}
			if _, err := client.Execute(context.Background(), "create_order", nil); err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if got := <-authorization; got != tt.wantAuthorization {
				t.Fatalf("Authorization = %q, want %q", got, tt.wantAuthorization)
			}
		})
	}
}
