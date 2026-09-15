// Package httpapi
package httpapi

import (
	"net/http"

	"github.com/dronm/webapp"
)

func BuildRoutes(dependencies ...Dependencies) []webapp.Route {
	var deps Dependencies
	if len(dependencies) > 0 {
		deps = dependencies[0]
	}

	routes := make([]webapp.Route, 0)
	api := webapp.NewGroup("/api", &routes)

	api.GET(
		"/health",
		webapp.WithHandler(func(w http.ResponseWriter, r *http.Request) {
			_ = webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		}),
	)

	applicationRouteRoutes(&api)

	registerGeneratedRoutes(&api)
	customerUserRoutes(&api)
	maxMiniAppRoutes(&api)
	maxNotificationRoutes(&api)
	notificationTemplateRoutes(&api)
	notificationTemplateRecipientRoutes(&api)
	orderDocumentRoutes(&api, deps)

	mainMenuRoutes(&api)

	objectHistoryRoutes(&api)

	integration1CRoutes(&api)
	integration1CJobRoutes(&api)

	progAboutRoutes(&api)

	userRoutes(&api)

	return routes
}
