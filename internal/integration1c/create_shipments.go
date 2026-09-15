package integration1c

import "context"

const CommandCreateShipments = "create_shipments"

type CreateShipmentResult struct {
	OrderID int    `json:"order_id"`
	ID      string `json:"id"`
	Descr   string `json:"descr"`
}

type CreateShipmentsResponse struct {
	Success bool                   `json:"success"`
	Payload []CreateShipmentResult `json:"payload"`
	Error   string                 `json:"error,omitempty"`
}

func (c *Client) CreateShipments(
	ctx context.Context,
	orderIDs []int,
) ([]CreateShipmentResult, error) {
	if err := ValidateOrderIDs(orderIDs); err != nil {
		return nil, err
	}

	return execute[CreateShipmentResult](ctx, c, CommandCreateShipments, OrderIDsParams{
		OrderIDs: append([]int(nil), orderIDs...),
	})
}
