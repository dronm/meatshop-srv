package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	"github.com/dronm/webapp/constants"
)

type constantTestSession struct {
	session.Session
	user models.UserLogin
	err  error
}

func (s constantTestSession) Get(key string, value any) error {
	if key != "user" {
		return fmt.Errorf("unexpected session key %q", key)
	}
	*value.(*models.UserLogin) = s.user
	return s.err
}

type constantEmptyRepository struct{}

func (constantEmptyRepository) LoadAll(context.Context) ([]constants.Record, error) {
	return nil, nil
}

func (constantEmptyRepository) Load(context.Context, []int64) ([]constants.Record, error) {
	return nil, nil
}

func (constantEmptyRepository) Update(context.Context, []constants.Write) ([]constants.Record, error) {
	return nil, fmt.Errorf("unexpected constants write")
}

func TestConstantServiceAuthorizationForDirectCalls(t *testing.T) {
	manager, err := constants.NewManager(constants.Config{
		Repository: constantEmptyRepository{},
		Registry:   constants.NewRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if err := RegisterConstantService(manager); err != nil {
		t.Fatal(err)
	}
	methods := []struct {
		name  string
		input any
	}{
		{"List", constants.ListOptions{}},
		{"Detail", constants.DetailInput{ID: 1}},
		{"Get", constants.GetInput{IDs: []int64{1}}},
		{"Set", constants.SetInput{Items: []constants.Change{{ID: 1, Value: json.RawMessage(`null`)}}}},
	}
	for _, test := range []struct {
		name    string
		session session.Session
		status  int
	}{
		{name: "anonymous", status: http.StatusUnauthorized},
		{name: "missing user", session: constantTestSession{err: errors.New("missing user")}, status: http.StatusUnauthorized},
		{name: "invalid user", session: constantTestSession{user: models.UserLogin{RoleID: models.RoleIDAdmin}}, status: http.StatusUnauthorized},
		{name: "customer", session: constantTestSession{user: models.UserLogin{ID: 1, RoleID: "customer"}}, status: http.StatusForbidden},
		{name: "banned admin", session: constantTestSession{user: models.UserLogin{ID: 2, RoleID: models.RoleIDAdmin, Banned: true}}, status: http.StatusUnauthorized},
		{name: "admin", session: constantTestSession{user: models.UserLogin{ID: 2, RoleID: models.RoleIDAdmin}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, method := range methods {
				// This is the direct dispatch path used by the WebSocket bridge:
				// session identity is supplied in ServiceContext, not the raw context.
				_, err := webapp.CallServiceMethod(context.Background(), "Constant", method.name, method.input, webapp.ServiceContext{Session: test.session})
				status := test.status
				if status == 0 {
					if method.name == "List" {
						if err != nil {
							t.Fatalf("admin %s: %v", method.name, err)
						}
						continue
					}
					// An empty catalogue produces 404 after the admin check passes.
					status = http.StatusNotFound
				}
				var httpErr webapp.HTTPError
				if !errors.As(err, &httpErr) || httpErr.StatusCode() != status {
					t.Fatalf("%s: status = %v, want %d", method.name, err, status)
				}
			}
		})
	}
	for _, body := range []string{`null`, `{"ids":[1],"user_id":2}`, `{"ids":[1]} {}`} {
		if _, err := webapp.DecodeInputForService("Constant", "Get", json.RawMessage(body)); err == nil {
			t.Fatalf("direct call accepted invalid input %s", body)
		}
	}
}
