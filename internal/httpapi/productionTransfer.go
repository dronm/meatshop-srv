package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/meatshop/internal/services"
	"github.com/dronm/webapp"
)

func productionTransferRoutes(api *webapp.Group, deps Dependencies) {
	api.GET("/production-transfers", webapp.WithName("productionTransfer.list"), webapp.WithPermission("productionTransfer.list"), webapp.WithService("ProductionTransfer", "List"), webapp.WithBinder(webapp.CollectionParamsBinder()))
	api.POST("/production-transfers/prepare", webapp.WithName("productionTransfer.prepare"), webapp.WithPermission("productionTransfer.create"), webapp.WithService("ProductionTransfer", "Prepare"), webapp.WithBinder(productionPrepareBinder()))
	api.GET("/production-transfers/order-options", webapp.WithName("productionTransfer.orderOptions"), webapp.WithPermission("order.list"), webapp.WithService("ProductionTransfer", "OrderOptions"), webapp.WithBinder(productionOrderOptionsBinder()))
	api.GET("/order/{id}/production-transfer", webapp.WithName("productionTransfer.byOrder"), webapp.WithPermission("productionTransfer.detail"), webapp.WithService("ProductionTransfer", "ByOrder"), webapp.WithBinder(webapp.PathValueBinder[int]("id")))
	api.POST("/production-transfers", webapp.WithName("productionTransfer.create"), webapp.WithPermission("productionTransfer.create"), webapp.WithService("ProductionTransfer", "Create"), webapp.WithBinder(productionDocumentBinder(false)), webapp.WithSuccessCode(http.StatusCreated))
	api.GET("/production-transfers/{id}", webapp.WithName("productionTransfer.detail"), webapp.WithPermission("productionTransfer.detail"), webapp.WithService("ProductionTransfer", "DocumentDetail"), webapp.WithBinder(webapp.PathValueBinder[int]("id")))
	api.PATCH("/production-transfers/{id}", webapp.WithName("productionTransfer.update"), webapp.WithPermission("productionTransfer.update"), webapp.WithService("ProductionTransfer", "Update"), webapp.WithBinder(productionDocumentBinder(true)))
	api.DELETE("/production-transfers/{id}", webapp.WithName("productionTransfer.delete"), webapp.WithPermission("productionTransfer.delete"), webapp.WithService("ProductionTransfer", "Delete"), webapp.WithBinder(webapp.PathValueBinder[int]("id")))
	api.POST("/production-transfers/{id}/retry-messages", webapp.WithName("productionTransfer.retryMessages"), webapp.WithPermission("productionTransfer.update"), webapp.WithService("ProductionTransfer", "RetryMessages"), webapp.WithBinder(webapp.PathValueBinder[int]("id")))
	api.GET("/production-transfers/{id}/print", webapp.WithName("productionTransfer.print"), webapp.WithPermission("productionTransfer.detail"), webapp.WithHandler(func(w http.ResponseWriter, r *http.Request) {
		id, err := positiveOrderDocumentPathID(r)
		if err != nil {
			webapp.WriteBindingError(w, webapp.BadRequest("Некорректный номер документа.", nil))
			return
		}
		data, err := services.GetProductionTransferPrint(r.Context(), deps.DB, id)
		if err != nil {
			webapp.WriteServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
}

func decodeProductionJSON(r *http.Request, value any) error {
	if r == nil || r.Body == nil {
		return webapp.BadRequest("Данные документа не переданы.", nil)
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 256<<10+1))
	if err != nil || len(data) == 0 || len(data) > 256<<10 {
		return webapp.BadRequest("Некорректный размер данных документа.", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return webapp.BadRequest("Некорректные данные передачи в производство.", nil)
	}
	if err := ensureOrderJSONEnd(decoder); err != nil {
		return webapp.BadRequest("Некорректные данные передачи в производство.", nil)
	}
	return nil
}
func productionPrepareBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		var input models.ProductionTransferPrepareRequest
		err := decodeProductionJSON(r, &input)
		return input, err
	}
}
func productionDocumentBinder(update bool) webapp.Binder {
	return func(r *http.Request) (any, error) {
		input := &models.ProductionTransferInput{}
		if err := decodeProductionJSON(r, input); err != nil {
			return nil, err
		}
		if !update {
			return input, nil
		}
		id, err := positiveOrderDocumentPathID(r)
		if err != nil {
			return nil, webapp.BadRequest("Некорректный номер документа.", nil)
		}
		return models.ProductionTransferUpdateRequest{ID: id, Document: input}, nil
	}
}
func productionOrderOptionsBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		input := models.ProductionOrderOptionsInput{Search: r.URL.Query().Get("search")}
		for name, target := range map[string]*int{"order_id": &input.OrderID, "from": &input.From, "count": &input.Count} {
			if raw := r.URL.Query().Get(name); raw != "" {
				value, err := strconv.Atoi(raw)
				if err != nil || value < 0 {
					return nil, webapp.BadRequest("Некорректные параметры списка заказов.", nil)
				}
				*target = value
			}
		}
		return input, nil
	}
}
