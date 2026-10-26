package models

// OrderStatusChangeRequest targets whole orders, including when their item rows
// were selected in the order-lines collection.
type OrderStatusChangeRequest struct {
	OrderIDs []int `json:"order_ids"`
	StatusID int   `json:"status_id"`
}
