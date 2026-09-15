package httpapi

import "github.com/dronm/webapp"

func customerUserRoutes(api *webapp.Group) {
	api.GET(
		"/customer-users",
		webapp.WithName("customerUser.list"),
		webapp.WithPermission("maxUser.list"),
		webapp.WithService("CustomerUser", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)
}
