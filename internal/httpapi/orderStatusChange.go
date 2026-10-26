package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const orderStatusChangeMaxBodySize = 64 << 10

func orderStatusChangeJSONBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		if r == nil || r.Body == nil {
			return nil, webapp.BadRequest("request body is required", nil)
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, orderStatusChangeMaxBodySize+1))
		if err != nil {
			return nil, webapp.BadRequest("read request body", nil)
		}
		if len(data) == 0 {
			return nil, webapp.BadRequest("request body is required", nil)
		}
		if len(data) > orderStatusChangeMaxBodySize {
			return nil, webapp.BadRequest("request body is too large", nil)
		}

		var input models.OrderStatusChangeRequest
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return nil, webapp.BadRequest("invalid order status JSON", nil)
		}
		if err := ensureOrderJSONEnd(decoder); err != nil {
			return nil, webapp.BadRequest("invalid order status JSON", nil)
		}
		return input, nil
	}
}
