package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func mainMenuRoutes(api *webapp.Group) {
	api.POST(
		"/main-menus",
		webapp.WithName("mainMenu.create"),
		webapp.WithPermission("mainMenu.create"),
		webapp.WithService("MainMenu", "Create"),
		webapp.WithBinder(webapp.InsertInputBinder[*models.MainMenu]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)

	api.GET(
		"/main-menus",
		webapp.WithName("mainMenu.list"),
		webapp.WithPermission("mainMenu.list"),
		webapp.WithService("MainMenu", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)

	api.GET(
		"/main-menus/for-user",
		webapp.WithName("mainMenu.for_user"),
		webapp.WithPermission("mainMenu.for_user"),
		webapp.WithService("MainMenu", "ForUser"),
	)

	api.GET(
		"/main-menus/routes/autocomplete",
		webapp.WithName("mainMenu.routeAutocomplete"),
		webapp.WithPermission("mainMenu.routeAutocomplete"),
		webapp.WithService("MainMenu", "RouteAutocomplete"),
		webapp.WithBinder(mainMenuRouteAutocompleteBinder()),
	)

	api.GET(
		"/main-menus/{id}",
		webapp.WithName("mainMenu.detail"),
		webapp.WithPermission("mainMenu.detail"),
		webapp.WithService("MainMenu", "Detail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)

	api.PATCH(
		"/main-menus/{id}",
		webapp.WithName("mainMenu.update"),
		webapp.WithPermission("mainMenu.update"),
		webapp.WithService("MainMenu", "Update"),
		webapp.WithBinder(webapp.UpdateByPathKeysBinder[*models.MainMenuKey, *models.MainMenu]("id")),
	)

	api.DELETE(
		"/main-menus/{id}",
		webapp.WithName("mainMenu.delete"),
		webapp.WithPermission("mainMenu.delete"),
		webapp.WithService("MainMenu", "Delete"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
}

func mainMenuRouteAutocompleteBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		limit := 20
		if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("main menu route autocomplete limit should be integer: %w", err)
			}
			limit = parsed
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			query = strings.TrimSpace(r.URL.Query().Get("query"))
		}

		return models.MainMenuRouteAutocompleteInput{
			Query: query,
			Limit: limit,
		}, nil
	}
}
