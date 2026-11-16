package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func orderLinesWeightConfirmationJSONBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		if r == nil || r.Body == nil {
			return nil, webapp.BadRequest("Тело запроса обязательно.", nil)
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, orderLineIDsMaxBodySize+1))
		if err != nil {
			return nil, webapp.BadRequest("Не удалось прочитать запрос подтверждения веса.", nil)
		}
		if len(data) == 0 {
			return nil, webapp.BadRequest("Тело запроса обязательно.", nil)
		}
		if len(data) > orderLineIDsMaxBodySize {
			return nil, webapp.BadRequest("Слишком большой запрос подтверждения веса.", nil)
		}

		var input models.OrderLinesWeightConfirmationRequest
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return nil, webapp.BadRequest("Некорректный запрос подтверждения веса.", nil)
		}
		if err := ensureOrderJSONEnd(decoder); err != nil {
			return nil, webapp.BadRequest("Некорректный запрос подтверждения веса.", nil)
		}
		if input.WeightConfirmed == nil {
			return nil, webapp.BadRequest("Укажите, нужно ли подтвердить вес выбранных позиций.", nil)
		}
		return input, nil
	}
}
