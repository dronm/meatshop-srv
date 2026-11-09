package models

// OrderLineIDsRequest identifies item rows rather than their parent orders.
type OrderLineIDsRequest struct {
	ItemIDs []int `json:"item_ids"`
}
