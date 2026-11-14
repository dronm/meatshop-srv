package services

import (
	"context"
	"net/http"
	"strings"

	"github.com/dronm/webapp"
	"github.com/dronm/webapp/events"
)

// EventService retains existing event behavior and restricts inbox events.
// Validate the whole request first so mixed subscriptions cannot partially apply.
type EventService struct {
	events.Service
	PermissionChecker webapp.PermissionChecker
}

func (s *EventService) Subscribe(ctx context.Context, input events.SubscribeRequest) error {
	for _, eventID := range input.Events {
		if strings.HasPrefix(eventID, "CustomerMessage.") {
			if _, err := requireCustomerMessageAdmin(s.Session); err != nil {
				return err
			}
			if s.PermissionChecker != nil {
				request := (&http.Request{}).WithContext(webapp.ContextWithSession(ctx, s.Session))
				if err := s.PermissionChecker(request, "customerMessage.list"); err != nil {
					return err
				}
			}
		}
	}
	return s.Service.Subscribe(ctx, input)
}
