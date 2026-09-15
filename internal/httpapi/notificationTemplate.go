package httpapi

import (
	"net/http"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func notificationTemplateRoutes(api *webapp.Group) {
	api.POST(
		"/notification-templates",
		webapp.WithName("notificationTemplate.create"),
		webapp.WithPermission("notificationTemplate.create"),
		webapp.WithService("NotificationTemplate", "Create"),
		webapp.WithBinder(webapp.InsertInputBinder[*models.NotificationTemplate]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.GET(
		"/notification-templates",
		webapp.WithName("notificationTemplate.list"),
		webapp.WithPermission("notificationTemplate.list"),
		webapp.WithService("NotificationTemplate", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)
	api.GET(
		"/notification-templates/{id}",
		webapp.WithName("notificationTemplate.detail"),
		webapp.WithPermission("notificationTemplate.detail"),
		webapp.WithService("NotificationTemplate", "Detail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.PATCH(
		"/notification-templates/{id}",
		webapp.WithName("notificationTemplate.update"),
		webapp.WithPermission("notificationTemplate.update"),
		webapp.WithService("NotificationTemplate", "Update"),
		webapp.WithBinder(webapp.UpdateByPathKeysBinder[*models.NotificationTemplateKey, *models.NotificationTemplate]("id")),
	)
	api.DELETE(
		"/notification-templates/{id}",
		webapp.WithName("notificationTemplate.delete"),
		webapp.WithPermission("notificationTemplate.delete"),
		webapp.WithService("NotificationTemplate", "Delete"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
}

func notificationTemplateRecipientRoutes(api *webapp.Group) {
	api.POST(
		"/notification-template-recipients",
		webapp.WithName("notificationTemplateRecipient.create"),
		webapp.WithPermission("notificationTemplateRecipient.create"),
		webapp.WithService("NotificationTemplateRecipient", "Create"),
		webapp.WithBinder(webapp.InsertInputBinder[*models.NotificationTemplateRecipient]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.GET(
		"/notification-template-recipients",
		webapp.WithName("notificationTemplateRecipient.list"),
		webapp.WithPermission("notificationTemplateRecipient.list"),
		webapp.WithService("NotificationTemplateRecipient", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)
	api.GET(
		"/notification-template-recipients/{id}",
		webapp.WithName("notificationTemplateRecipient.detail"),
		webapp.WithPermission("notificationTemplateRecipient.detail"),
		webapp.WithService("NotificationTemplateRecipient", "Detail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.PATCH(
		"/notification-template-recipients/{id}",
		webapp.WithName("notificationTemplateRecipient.update"),
		webapp.WithPermission("notificationTemplateRecipient.update"),
		webapp.WithService("NotificationTemplateRecipient", "Update"),
		webapp.WithBinder(webapp.UpdateByPathKeysBinder[*models.NotificationTemplateRecipientKey, *models.NotificationTemplateRecipient]("id")),
	)
	api.DELETE(
		"/notification-template-recipients/{id}",
		webapp.WithName("notificationTemplateRecipient.delete"),
		webapp.WithPermission("notificationTemplateRecipient.delete"),
		webapp.WithService("NotificationTemplateRecipient", "Delete"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
}
