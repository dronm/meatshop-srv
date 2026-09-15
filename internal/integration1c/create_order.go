package integration1c

const CommandCreateOrder = "create_order"

type CreateOrderParams struct {
	OrderID      int                  `json:"order_id"`
	OrderVersion int64                `json:"order_version"`
	CustomerID   string               `json:"customer_id"`
	Products     []CreateOrderProduct `json:"products"`
}

type CreateOrderProduct struct {
	ID    string  `json:"id"`
	Quant float64 `json:"quant"`
}

type CreateOrderResult struct {
	ID       string `json:"id"`
	Descr    string `json:"descr"`
	Number1C string `json:"number_1c"`
}

type CreateOrderResponse struct {
	Success bool               `json:"success"`
	Payload *CreateOrderResult `json:"payload,omitempty"`
	Error   string             `json:"error,omitempty"`
}
