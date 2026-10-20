package integration1c

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdminEndpoints(t *testing.T) {
	commands := []struct {
		name   string
		method string
		call   func(*Client, context.Context) (AdminResponse, error)
	}{
		{"health", http.MethodGet, (*Client).Health},
		{"status", http.MethodGet, (*Client).Status},
		{"stop", http.MethodPost, (*Client).Stop},
		{"start", http.MethodPost, (*Client).Start},
	}
	for _, command := range commands {
		for _, suffix := range []string{"", "/", "/execute", "/bin-data"} {
			for _, auth := range []*Auth{nil, {UserName: "admin", Password: "secret"}} {
				t.Run(command.name+suffix+"/"+func() string {
					if auth == nil {
						return "no-auth"
					}
					return "basic-auth"
				}(), func(t *testing.T) {
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						if r.Method != command.method || r.URL.Path != "/gateway/"+command.name {
							t.Errorf("request = %s %s", r.Method, r.URL.Path)
						}
						if auth != nil && command.name != "health" {
							user, password, ok := r.BasicAuth()
							if !ok || user != auth.UserName || password != auth.Password {
								t.Error("missing or incorrect basic authentication")
							}
						} else if r.Header.Get("Authorization") != "" {
							t.Error("unexpected Authorization header")
						}
						w.WriteHeader(http.StatusServiceUnavailable)
						_, _ = w.Write([]byte(`{"success":false,"error":"stopped"}`))
					}))
					defer server.Close()
					client, err := NewClient(Config{URL: server.URL + "/gateway" + suffix, Auth: auth, MaxRetries: 3})
					if err != nil {
						t.Fatal(err)
					}
					response, err := command.call(client, context.Background())
					if err != nil || response.StatusCode != http.StatusServiceUnavailable {
						t.Fatalf("response = %+v, err = %v", response, err)
					}
					body, err := json.Marshal(response.Body)
					if err != nil || string(body) != `{"success":false,"error":"stopped"}` {
						t.Fatalf("body = %s, err = %v", body, err)
					}
					if calls.Load() != 1 {
						t.Fatal("admin request was retried")
					}
				})
			}
		}
	}
}

func TestAdminDoesNotFollowRedirect(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := NewClient(Config{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Stop(context.Background())
	if err != nil || response.StatusCode != http.StatusTemporaryRedirect || calls.Load() != 1 {
		t.Fatalf("response = %+v, calls = %d, err = %v", response, calls.Load(), err)
	}
}

func TestAdminTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := NewClient(Config{URL: server.URL, Timeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(context.Background()); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestAdminTextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("authentication required"))
	}))
	defer server.Close()
	client, err := NewClient(Config{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Status(context.Background())
	if err != nil || response.StatusCode != http.StatusUnauthorized || response.Body != "authentication required" {
		t.Fatalf("response = %+v, err = %v", response, err)
	}
}
