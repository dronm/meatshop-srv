package integration1c

const CommandCreateOrder = "create_order"

type CreateOrderParams struct {
	OrderID      int                  `json:"order_id"`
	OrderVersion int64                `json:"order_version"`
	CustomerID   string               `json:"customer_id"`
	Products     []CreateOrderProduct `json:"products"`
}

type CreateOrderProduct struct {
	ID          string  `json:"id"`
	Quant       float64 `json:"quant"`
	OrderItemID int     `json:"order_item_id"`
}

type CreateOrderResult struct {
	ID       string                  `json:"id"`
	Descr    string                  `json:"descr"`
	Number1C string                  `json:"number_1c"`
	Items    []CreateOrderItemResult `json:"items,omitempty"`
}

// Monetary values are decimal strings so the 1C response does not pass
// through a binary floating-point representation before reaching NUMERIC.
type CreateOrderItemResult struct {
	OrderItemID int    `json:"order_item_id"`
	Price       string `json:"price"`
	Amount      string `json:"amount"`
	VatPercent  string `json:"vat_percent"`
	VatAmount   string `json:"vat_amount"`
	UseMarking  *bool  `json:"use_marking"`
}

type CreateOrderResponse struct {
	Success bool               `json:"success"`
	Payload *CreateOrderResult `json:"payload,omitempty"`
	Error   string             `json:"error,omitempty"`
}
