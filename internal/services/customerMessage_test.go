package services

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/webapp/events"
)

type customerMessageTestSession struct {
	session.Session
	user models.UserLogin
}

func (s customerMessageTestSession) Get(_ string, value any) error {
	*value.(*models.UserLogin) = s.user
	return nil
}

func TestCustomerMessageReadSelection(t *testing.T) {
	input := models.CustomerMessageMarkReadRequest{MessageIDs: []int64{9, 3, 9}}
	ids, err := validatedCustomerMessageIDs(input)
	if err != nil || !reflect.DeepEqual(ids, []int64{3, 9}) || input.MessageIDs[0] != 9 {
		t.Fatalf("selection = %v, input = %v, err = %v", ids, input.MessageIDs, err)
	}
	for _, ids := range [][]int64{nil, {0}, {-1, 3}, make([]int64, 501)} {
		if _, err := validatedCustomerMessageIDs(models.CustomerMessageMarkReadRequest{MessageIDs: ids}); err == nil {
			t.Fatalf("accepted invalid selection %v", ids)
		}
	}
}

func TestCustomerMessageCollectionSQL(t *testing.T) {
	params := modelbind.CollectionParams{Filter: []modelbind.CollectionFilter{{
		Fields: map[string]modelbind.CollectionFilterField{
			"customer_id": {Operator: modelbind.FilterOperParEq, Value: 17},
			"is_read":     {Operator: modelbind.FilterOperParEq, Value: false},
		},
	}}}
	query, total, args, err := customerMessageCollectionSQL(42, params)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{query, total} {
		if !strings.Contains(sql, "r.user_id = $1") || !strings.Contains(sql, "customer_id") || !strings.Contains(sql, "is_read") {
			t.Fatalf("missing user scope or filters: %s", sql)
		}
	}
	if args[0] != 42 || len(args) != 3 || !strings.Contains(query, "ORDER BY id DESC") {
		t.Fatalf("query = %s, args = %v", query, args)
	}
	for _, params := range []modelbind.CollectionParams{
		{From: -1}, {Count: -1}, {Count: 201},
		{Filter: []modelbind.CollectionFilter{{Fields: map[string]modelbind.CollectionFilterField{
			"user_id": {Operator: modelbind.FilterOperParEq, Value: 99},
		}}}},
		{Sorter: []modelbind.CollectionSorter{{Field: "id; DROP TABLE users"}}},
	} {
		if _, _, _, err := customerMessageCollectionSQL(42, params); err == nil {
			t.Fatalf("accepted invalid params %#v", params)
		}
	}
}

type customerMessageEventHandler struct {
	events.Handler
	events []string
}

func (h *customerMessageEventHandler) SubscribeToEvent(_ string, id string) error {
	h.events = append(h.events, id)
	return nil
}

func (s customerMessageTestSession) SessionID() string { return "customer-message-test" }

func TestCustomerMessageEventAuthorization(t *testing.T) {
	for _, test := range []struct {
		name    string
		user    models.UserLogin
		allowed bool
	}{
		{name: "anonymous"},
		{name: "customer", user: models.UserLogin{ID: 1, RoleID: "customer"}},
		{name: "banned admin", user: models.UserLogin{ID: 2, RoleID: models.RoleIDAdmin, Banned: true}},
		{name: "admin", user: models.UserLogin{ID: 2, RoleID: models.RoleIDAdmin}, allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := &customerMessageEventHandler{}
			svc := &EventService{Service: events.Service{
				Session: customerMessageTestSession{user: test.user}, Handler: handler,
			}}
			err := svc.Subscribe(context.Background(), events.SubscribeRequest{
				Events: []string{"Order.Update", "CustomerMessage.Create", "CustomerMessage.Read"},
			})
			if (err == nil) != test.allowed || !test.allowed && len(handler.events) != 0 {
				t.Fatalf("allowed = %v, err = %v, subscriptions = %v", test.allowed, err, handler.events)
			}
			if test.allowed && len(handler.events) != 3 {
				t.Fatalf("subscriptions = %v", handler.events)
			}
		})
	}
}
