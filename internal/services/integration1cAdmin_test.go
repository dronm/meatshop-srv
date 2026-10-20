package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/session"
)

type integrationAdminSession struct {
	session.Session
	user models.UserLogin
	err  error
}

func (s integrationAdminSession) Get(key string, value any) error {
	if key != "user" {
		return fmt.Errorf("unexpected session key %q", key)
	}
	*value.(*models.UserLogin) = s.user
	return s.err
}

func TestIntegration1CAdminAccess(t *testing.T) {
	commands := []func(*Integration1CService, context.Context) (integration.AdminResponse, error){
		(*Integration1CService).Health,
		(*Integration1CService).Status,
		(*Integration1CService).Stop,
		(*Integration1CService).Start,
	}
	for _, test := range []struct {
		name    string
		session session.Session
		allowed bool
	}{
		{name: "anonymous"},
		{name: "missing user", session: integrationAdminSession{err: fmt.Errorf("missing user")}},
		{name: "invalid user", session: integrationAdminSession{user: models.UserLogin{RoleID: models.RoleIDAdmin}}},
		{name: "non-admin", session: integrationAdminSession{user: models.UserLogin{ID: 1, RoleID: "customer"}}},
		{name: "admin", session: integrationAdminSession{user: models.UserLogin{ID: 1, RoleID: models.RoleIDAdmin}}, allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer server.Close()
			client, err := integration.NewClient(integration.Config{URL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			service := &Integration1CService{Session: test.session, Client: client}
			for _, command := range commands {
				_, err := command(service, context.Background())
				if (err == nil) != test.allowed {
					t.Fatalf("allowed = %t, err = %v", test.allowed, err)
				}
			}
			if test.allowed && calls.Load() != int32(len(commands)) || !test.allowed && calls.Load() != 0 {
				t.Fatalf("gateway calls = %d", calls.Load())
			}
		})
	}
}

func TestIntegration1CAdminNotConfigured(t *testing.T) {
	service := &Integration1CService{Session: integrationAdminSession{user: models.UserLogin{ID: 1, RoleID: models.RoleIDAdmin}}}
	if _, err := service.Status(context.Background()); err == nil {
		t.Fatal("expected configuration error")
	}
}
