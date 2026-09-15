package httpapi

import (
	"github.com/dronm/webapp"
)

func progAboutRoutes(api *webapp.Group) {
	api.GET(
		"/prog-about/info",
		webapp.WithName("progAbout.info"),
		webapp.WithPermission("progAbout.info"),
		webapp.WithService("ProgAbout", "Info"),
	)
}

