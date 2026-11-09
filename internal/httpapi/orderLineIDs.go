package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const orderLineIDsMaxBodySize = 64 << 10

func orderLineIDsJSONBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		if r == nil || r.Body == nil {
			return nil, webapp.BadRequest("request body is required", nil)
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, orderLineIDsMaxBodySize+1))
		if err != nil {
			return nil, webapp.BadRequest("read request body", nil)
		}
		if len(data) == 0 {
			return nil, webapp.BadRequest("request body is required", nil)
		}
		if len(data) > orderLineIDsMaxBodySize {
			return nil, webapp.BadRequest("request body is too large", nil)
		}

		var input models.OrderLineIDsRequest
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return nil, webapp.BadRequest("invalid order line selection JSON", nil)
		}
		if err := ensureOrderJSONEnd(decoder); err != nil {
			return nil, webapp.BadRequest("invalid order line selection JSON", nil)
		}
		return input, nil
	}
}
