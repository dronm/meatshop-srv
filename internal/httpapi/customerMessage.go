package httpapi

import (
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func customerMessageActionRoutes(api *webapp.Group) {
	api.GET(
		"/customer-messages/unread-count",
		webapp.WithName("customerMessage.unreadCount"),
		webapp.WithPermission("customerMessage.list"),
		webapp.WithService("CustomerMessage", "UnreadCount"),
		webapp.WithBinder(maxOptionalQueryIntBinder("customer_id")),
	)
	api.POST(
		"/customer-messages/mark-read",
		webapp.WithName("customerMessage.markRead"),
		webapp.WithPermission("customerMessage.markRead"),
		webapp.WithService("CustomerMessage", "MarkRead"),
		webapp.WithBinder(maxJSONBinder[models.CustomerMessageMarkReadRequest]()),
	)
}
