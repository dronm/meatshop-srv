package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const orderDocumentMaxBodySize = 2 << 20

func orderDocumentRoutes(api *webapp.Group, deps Dependencies) {
	api.GET(
		"/order",
		webapp.WithName("order.list"),
		webapp.WithPermission("order.list"),
		webapp.WithService("Order", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)
	api.POST(
		"/order",
		webapp.WithName("order.create"),
		webapp.WithPermission("order.create"),
		webapp.WithService("Order", "Create"),
		webapp.WithBinder(orderDocumentJSONBinder()),
		webapp.WithSuccessCode(http.StatusCreated),
	)
	api.POST(
		"/order/print-1c",
		webapp.WithName("order.print1c.batch"),
		webapp.WithPermission("order.print1c"),
		webapp.WithHandler(orderBatchPrint1CHandler(deps.DB, deps.Integration1CClient)),
	)
	api.POST(
		"/order/create-shipments-1c",
		webapp.WithName("order.createShipments1c"),
		webapp.WithPermission("order.createShipments1c"),
		webapp.WithService("Order", "CreateShipments1C"),
		webapp.WithBinder(orderIDsJSONBinder()),
		webapp.WithSuccessCode(http.StatusAccepted),
	)
	api.POST(
		"/order/print-shipment-1c",
		webapp.WithName("order.printShipment1c"),
		webapp.WithPermission("order.printShipment1c"),
		webapp.WithHandler(shipmentBatchPrint1CHandler(deps.DB, deps.Integration1CClient)),
	)
	api.GET(
		"/order/{id}",
		webapp.WithName("order.detail"),
		webapp.WithPermission("order.detail"),
		webapp.WithService("Order", "DocumentDetail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.PUT(
		"/order/{id}",
		webapp.WithName("order.update"),
		webapp.WithPermission("order.update"),
		webapp.WithService("Order", "Update"),
		webapp.WithBinder(orderDocumentUpdateBinder()),
	)
	api.DELETE(
		"/order/{id}",
		webapp.WithName("order.delete"),
		webapp.WithPermission("order.delete"),
		webapp.WithService("Order", "Delete"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)
	api.POST(
		"/order/{id}/create-1c",
		webapp.WithName("order.create1c"),
		webapp.WithPermission("order.create1c"),
		webapp.WithService("Order", "Create1C"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
		webapp.WithSuccessCode(http.StatusAccepted),
	)
	api.GET(
		"/order/{id}/print-1c",
		webapp.WithName("order.print1c"),
		webapp.WithPermission("order.print1c"),
		webapp.WithHandler(orderPrint1CHandler(deps.DB, deps.Integration1CClient)),
	)
}

func orderDocumentJSONBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		return decodeOrderDocumentJSON(r)
	}
}

func orderDocumentUpdateBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		id, err := positiveOrderDocumentPathID(r)
		if err != nil {
			return nil, err
		}
		document, err := decodeOrderDocumentJSON(r)
		if err != nil {
			return nil, err
		}
		return models.UpdateOrderDocumentRequest{ID: id, Document: document}, nil
	}
}

func decodeOrderDocumentJSON(r *http.Request) (*models.OrderDocument, error) {
	if r == nil || r.Body == nil {
		return nil, webapp.BadRequest("order request body is required", nil)
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, orderDocumentMaxBodySize+1))
	if err != nil {
		return nil, webapp.BadRequest("read order request body", nil)
	}
	if len(data) == 0 {
		return nil, webapp.BadRequest("order request body is required", nil)
	}
	if len(data) > orderDocumentMaxBodySize {
		return nil, webapp.BadRequest("order request body is too large", nil)
	}

	var document models.OrderDocument
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, webapp.BadRequest("invalid order JSON", map[string]any{"error": err.Error()})
	}
	if err := ensureOrderJSONEnd(decoder); err != nil {
		return nil, webapp.BadRequest("invalid order JSON", map[string]any{"error": err.Error()})
	}

	return &document, nil
}

func ensureOrderJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("request body contains more than one JSON value")
}

func positiveOrderDocumentPathID(r *http.Request) (int, error) {
	value := r.PathValue("id")
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, webapp.BadRequest("order id should be a positive integer", map[string]any{"id": value})
	}
	return id, nil
}
