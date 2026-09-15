package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/meatshop/internal/services"
	"github.com/dronm/webapp"
)

const (
	order1CActionMaxBodySize = 64 << 10
	order1CActionMaxOrders   = 1000
)

func orderPrint1CHandler(db ds.Provider, client *integration1c.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := positiveOrderDocumentPathID(r)
		if err != nil {
			webapp.WriteBindingError(w, err)
			return
		}

		pdf, err := services.GetOrderPrint1C(r.Context(), db, client, []int{id})
		if err != nil {
			webapp.WriteServiceError(w, err)
			return
		}

		writeOrder1CPDF(w, pdf, fmt.Sprintf("order-%d.pdf", id))
	}
}

func orderBatchPrint1CHandler(
	db ds.Provider,
	client *integration1c.Client,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input, err := decodeOrderIDsJSON(r)
		if err != nil {
			webapp.WriteBindingError(w, err)
			return
		}

		pdf, err := services.GetOrderPrint1C(r.Context(), db, client, input.OrderIDs)
		if err != nil {
			webapp.WriteServiceError(w, err)
			return
		}

		writeOrder1CPDF(w, pdf, "orders.pdf")
	}
}

func shipmentBatchPrint1CHandler(
	db ds.Provider,
	client *integration1c.Client,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input, err := decodeOrderIDsJSON(r)
		if err != nil {
			webapp.WriteBindingError(w, err)
			return
		}

		pdf, err := services.GetShipmentPrint1C(r.Context(), db, client, input.OrderIDs)
		if err != nil {
			webapp.WriteServiceError(w, err)
			return
		}

		writeOrder1CPDF(w, pdf, "shipments.pdf")
	}
}

func orderIDsJSONBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		return decodeOrderIDsJSON(r)
	}
}

func decodeOrderIDsJSON(r *http.Request) (models.OrderIDsRequest, error) {
	if r == nil || r.Body == nil {
		return models.OrderIDsRequest{}, webapp.BadRequest("request body is required", nil)
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, order1CActionMaxBodySize+1))
	if err != nil {
		return models.OrderIDsRequest{}, webapp.BadRequest("read request body", nil)
	}
	if len(data) == 0 {
		return models.OrderIDsRequest{}, webapp.BadRequest("request body is required", nil)
	}
	if len(data) > order1CActionMaxBodySize {
		return models.OrderIDsRequest{}, webapp.BadRequest("request body is too large", nil)
	}

	var input models.OrderIDsRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return models.OrderIDsRequest{}, webapp.BadRequest(
			"invalid order ids JSON",
			map[string]any{"error": err.Error()},
		)
	}
	if err := ensureOrderJSONEnd(decoder); err != nil {
		return models.OrderIDsRequest{}, webapp.BadRequest(
			"invalid order ids JSON",
			map[string]any{"error": err.Error()},
		)
	}
	if len(input.OrderIDs) > order1CActionMaxOrders {
		return models.OrderIDsRequest{}, webapp.BadRequest(
			"too many order ids",
			map[string]any{
				"count":   len(input.OrderIDs),
				"maximum": order1CActionMaxOrders,
			},
		)
	}
	if err := integration1c.ValidateOrderIDs(input.OrderIDs); err != nil {
		return models.OrderIDsRequest{}, webapp.BadRequest(err.Error(), nil)
	}

	input.OrderIDs = append([]int(nil), input.OrderIDs...)
	return input, nil
}

func writeOrder1CPDF(
	w http.ResponseWriter,
	pdf integration1c.BinaryResponse,
	fallbackFileName string,
) {
	fileName := strings.TrimSpace(pdf.FileName)
	if fileName == "" {
		fileName = fallbackFileName
	}
	fileName = filepath.Base(fileName)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set(
		"Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": fileName}),
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf.Body)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf.Body)
}
