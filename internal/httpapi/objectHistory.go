package httpapi

import (
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func objectHistoryRoutes(api *webapp.Group) {
	api.GET(
		"/object-history",
		webapp.WithName("objectHistory.list"),
		webapp.WithPermission("objectHistory.list"),
		webapp.WithService("ObjectHistory", "List"),
		webapp.WithBinder(objectHistoryBinder()),
	)
}

func objectHistoryBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input, err := modelbind.DecodeURLValuesInput[*models.ObjectHistoryQuery](r.URL.Query())
		if err != nil {
			return nil, err
		}
		if err := input.Validate(true); err != nil {
			return nil, err
		}

		params, err := webapp.ParseCollectionParams(r)
		if err != nil {
			return nil, err
		}

		return models.ObjectHistoryInput{
			Query:  input.Model,
			Params: params,
		}, nil
	}
}
