package integration1c

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"strings"
)

const (
	CommandPrintOrder    = "print_order"
	CommandPrintShipment = "print_shipment"
)

type OrderIDsParams struct {
	OrderIDs []int `json:"order_ids"`
}

func (c *Client) PrintOrder(ctx context.Context, orderIDs []int) (BinaryResponse, error) {
	return c.printPDF(ctx, CommandPrintOrder, "order", orderIDs)
}

func (c *Client) PrintShipment(ctx context.Context, orderIDs []int) (BinaryResponse, error) {
	return c.printPDF(ctx, CommandPrintShipment, "shipment", orderIDs)
}

func (c *Client) printPDF(
	ctx context.Context,
	command string,
	documentName string,
	orderIDs []int,
) (BinaryResponse, error) {
	if err := ValidateOrderIDs(orderIDs); err != nil {
		return BinaryResponse{}, err
	}

	response, err := executeBinary(ctx, c, command, OrderIDsParams{
		OrderIDs: append([]int(nil), orderIDs...),
	})
	if err != nil {
		return BinaryResponse{}, err
	}

	if !isPDFResponse(response) {
		return BinaryResponse{}, fmt.Errorf(
			"1c %s print returned unexpected content type %q",
			documentName,
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

func ValidateOrderIDs(orderIDs []int) error {
	if len(orderIDs) == 0 {
		return fmt.Errorf("order_ids is required")
	}

	seen := make(map[int]struct{}, len(orderIDs))
	for index, orderID := range orderIDs {
		if orderID <= 0 {
			return fmt.Errorf("order_ids[%d] should be positive", index)
		}
		if _, exists := seen[orderID]; exists {
			return fmt.Errorf("order_ids[%d] is duplicated", index)
		}
		seen[orderID] = struct{}{}
	}

	return nil
}
