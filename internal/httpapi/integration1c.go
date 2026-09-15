package httpapi

import (
	"net/http"
	"strings"

	"github.com/dronm/webapp"
)

func integration1CRoutes(api *webapp.Group) {
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
