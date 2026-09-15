package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const maxMiniAppMaxBodySize = 1 << 20

func maxMiniAppRoutes(api *webapp.Group) {
	api.POST(
		"/max/session",
		webapp.WithName("max.session"),
		webapp.WithService("MaxMiniApp", "SessionStart"),
		webapp.WithBinder(maxJSONBinder[models.MaxSessionRequest]()),
	)
	api.GET(
		"/max/me",
		webapp.WithName("max.me"),
		webapp.WithService("MaxMiniApp", "Me"),
	)
	api.POST(
		"/max/registration/customer",
		webapp.WithName("max.registration.customer"),
		webapp.WithService("MaxMiniApp", "FindCustomer"),
		webapp.WithBinder(maxCustomerLookupBinder()),
	)
	api.GET(
		"/max/registration/sale-places",
		webapp.WithName("max.registration.salePlaces"),
		webapp.WithService("MaxMiniApp", "SalePlaces"),
		webapp.WithBinder(maxPositiveQueryIntBinder("customer_id")),
	)
	api.GET(
		"/max/sale-places",
		webapp.WithName("max.salePlaces"),
		webapp.WithService("MaxMiniApp", "CustomerSalePlaces"),
	)
	api.POST(
		"/max/registration",
		webapp.WithName("max.registration.register"),
		webapp.WithService("MaxMiniApp", "Register"),
		webapp.WithBinder(maxJSONBinder[models.MaxRegistrationRequest]()),
	)
	api.GET(
		"/max/catalog",
		webapp.WithName("max.catalog"),
		webapp.WithService("MaxMiniApp", "Catalogue"),
		webapp.WithBinder(maxCatalogueScopeBinder()),
	)
	api.GET(
		"/max/orders",
		webapp.WithName("max.orders.list"),
		webapp.WithService("MaxMiniApp", "Orders"),
		webapp.WithBinder(maxOptionalQueryIntBinder("last_id")),
	)
	api.GET(
		"/max/order-date-limits",
		webapp.WithName("max.orders.dateLimits"),
		webapp.WithService("MaxMiniApp", "OrderDateLimits"),
	)
	api.GET(
		"/max/orders/{id}",
		webapp.WithName("max.orders.detail"),
		webapp.WithService("MaxMiniApp", "OrderDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.POST(
		"/max/orders",
		webapp.WithName("max.orders.create"),
		webapp.WithService("MaxMiniApp", "CreateOrder"),
		webapp.WithBinder(maxJSONBinder[models.MaxOrderSubmitRequest]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.PUT(
		"/max/orders/{id}",
		webapp.WithName("max.orders.update"),
		webapp.WithService("MaxMiniApp", "UpdateOrder"),
		webapp.WithBinder(maxOrderUpdateBinder()),
	)
}

func maxJSONBinder[T any]() webapp.Binder {
	return func(r *http.Request) (any, error) {
		if r == nil || r.Body == nil {
			return nil, webapp.BadRequest("request body is required", nil)
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, maxMiniAppMaxBodySize+1))
		if err != nil {
			return nil, webapp.BadRequest("read request body", nil)
		}
		if len(data) == 0 {
			return nil, webapp.BadRequest("request body is required", nil)
		}
		if len(data) > maxMiniAppMaxBodySize {
			return nil, webapp.BadRequest("request body is too large", nil)
		}
		var input T
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return nil, webapp.BadRequest("invalid JSON", map[string]any{"error": err.Error()})
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, webapp.BadRequest("request body should contain one JSON value", nil)
		}
		return input, nil
	}
}

func maxCustomerLookupBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		bound, err := maxJSONBinder[models.MaxCustomerLookupRequest]()(r)
		if err != nil {
			return nil, err
		}

		input := bound.(models.MaxCustomerLookupRequest)
		input.INN = strings.TrimSpace(input.INN)
		if input.INN == "" {
			return nil, webapp.BadRequest("inn is required", nil)
		}
		input.AppUsername = strings.TrimSpace(input.AppUsername)
		if input.AppUsername == "" {
			return nil, webapp.BadRequest("app_username is required", nil)
		}

		return input, nil
	}
}

func maxPositiveQueryIntBinder(name string) webapp.Binder {
	return func(r *http.Request) (any, error) {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return nil, webapp.BadRequest(name+" query parameter should be a positive integer", nil)
		}
		return parsed, nil
	}
}

func maxOptionalQueryIntBinder(name string) webapp.Binder {
	return func(r *http.Request) (any, error) {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			var result *int
			return result, nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, webapp.BadRequest(name+" query parameter should be an integer", nil)
		}
		return &parsed, nil
	}
}

func maxCatalogueScopeBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		scope := models.MaxCatalogueScope(strings.TrimSpace(r.URL.Query().Get("scope")))
		if scope == "" {
			scope = models.MaxCatalogueScopeAll
		}
		return scope, nil
	}
}

func maxOrderUpdateBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveOrderDocumentPathID(r)
		if err != nil {
			return nil, err
		}
		bound, err := maxJSONBinder[models.MaxOrderUpdateRequest]()(r)
		if err != nil {
			return nil, err
		}
		input := bound.(models.MaxOrderUpdateRequest)
		input.ID = id
		return input, nil
	}
}
