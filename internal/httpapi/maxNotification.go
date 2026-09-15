package httpapi

import (
	"net/http"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func maxNotificationRoutes(api *webapp.Group) {
	api.POST(
		"/max/notifications",
		webapp.WithName("max.notifications.send"),
		webapp.WithPermission("maxNotification.send"),
		webapp.WithService("MaxNotification", "Send"),
		webapp.WithBinder(maxJSONBinder[models.MaxNotificationSendRequest]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
}
