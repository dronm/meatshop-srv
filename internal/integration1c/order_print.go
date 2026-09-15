package integration1c

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"strings"
)

const CommandOrderPrintForm = "order_print_form"

type OrderPrintFormParams struct {
	OrderID string `json:"order_id"`
}

func (c *Client) OrderPrintForm(ctx context.Context, orderID string) (BinaryResponse, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return BinaryResponse{}, fmt.Errorf("1c order id is required")
	}

	response, err := executeBinary(ctx, c, CommandOrderPrintForm, OrderPrintFormParams{
		OrderID: orderID,
	})
	if err != nil {
		return BinaryResponse{}, err
	}

	if !isPDFResponse(response) {
		return BinaryResponse{}, fmt.Errorf(
			"1c order print form returned unexpected content type %q",
			response.ContentType,
		)
	}
	response.ContentType = "application/pdf"
	return response, nil
}

func isPDFResponse(response BinaryResponse) bool {
	mediaType := strings.TrimSpace(response.ContentType)
	if parsedType, _, err := mime.ParseMediaType(mediaType); err == nil {
		mediaType = parsedType
	}
	if strings.EqualFold(mediaType, "application/pdf") {
		return true
	}

	return bytes.HasPrefix(bytes.TrimSpace(response.Body), []byte("%PDF-"))
}
