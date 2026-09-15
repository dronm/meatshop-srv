package httpapi

import "github.com/dronm/webapp"

func integration1CJobRoutes(api *webapp.Group) {
	api.GET(
		"/integration-1c/jobs",
		webapp.WithName("integration1c.jobs.list"),
		webapp.WithPermission("integration1c.jobs.list"),
		webapp.WithService("Integration1CJob", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)
}
