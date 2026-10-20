package httpapi

import (
	"net/http"
	"strings"

	"github.com/dronm/webapp"
)

func integration1CRoutes(api *webapp.Group) {
	api.GET(
		"/integration-1c/health",
		webapp.WithName("integration1c.admin.health"),
		webapp.WithPermission("integration1c.admin"),
		webapp.WithService("Integration1C", "Health"),
	)
	api.GET(
		"/integration-1c/status",
		webapp.WithName("integration1c.admin.status"),
		webapp.WithPermission("integration1c.admin"),
		webapp.WithService("Integration1C", "Status"),
	)
	api.POST(
		"/integration-1c/stop",
		webapp.WithName("integration1c.admin.stop"),
		webapp.WithPermission("integration1c.admin"),
		webapp.WithService("Integration1C", "Stop"),
	)
	api.POST(
		"/integration-1c/start",
		webapp.WithName("integration1c.admin.start"),
		webapp.WithPermission("integration1c.admin"),
		webapp.WithService("Integration1C", "Start"),
	)

	api.GET(
		"/integration-1c/nomenclature",
		webapp.WithName("integration1c.nomenclature.complete"),
		webapp.WithPermission("integration1c.nomenclature.complete"),
		webapp.WithService("Integration1C", "CompleteNomenclature"),
		webapp.WithBinder(integration1CNameBinder()),
	)

	api.GET(
		"/integration-1c/counterparties",
		webapp.WithName("integration1c.counterparty.complete"),
		webapp.WithPermission("integration1c.counterparty.complete"),
		webapp.WithService("Integration1C", "CompleteCounterparties"),
		webapp.WithBinder(integration1CNameBinder()),
	)
}

func integration1CNameBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			return nil, webapp.BadRequest("name query parameter is required", nil)
		}
		return name, nil
	}
}
