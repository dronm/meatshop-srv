package httpapi

import (
	"fmt"
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func applicationRouteRoutes(api *webapp.Group) {
	api.GET(
		"/application-routes",
		webapp.WithName("applicationRoute.list"),
		webapp.WithPermission("applicationRoute.list"),
		webapp.WithService("ApplicationRoute", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)

	api.POST(
		"/application-routes/sync",
		webapp.WithName("applicationRoute.sync"),
		webapp.WithPermission("applicationRoute.sync"),
		webapp.WithService("ApplicationRoute", "Sync"),
		webapp.WithBinder(applicationRouteSyncBinder()),
	)

	api.GET(
		"/application-routes/{id}",
		webapp.WithName("applicationRoute.detail"),
		webapp.WithPermission("applicationRoute.detail"),
		webapp.WithService("ApplicationRoute", "Detail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)

	api.PATCH(
		"/application-routes/{id}",
		webapp.WithName("applicationRoute.update"),
		webapp.WithPermission("applicationRoute.update"),
		webapp.WithService("ApplicationRoute", "Update"),
		webapp.WithBinder(webapp.UpdateByPathKeysBinder[*models.ApplicationRouteKey, *models.ApplicationRoute]("id")),
	)
}

func applicationRouteSyncBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		bodyInput, err := modelbind.DecodeRequestInput[*models.ApplicationRouteSyncInput](r)
		if err != nil {
			return nil, fmt.Errorf("decode application route sync request: %w", err)
		}
		if bodyInput.Model == nil {
			return nil, fmt.Errorf("application route sync request is empty")
		}
		return *bodyInput.Model, nil
	}
}
