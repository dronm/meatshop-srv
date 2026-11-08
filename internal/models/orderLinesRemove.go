package models

// OrderLinesRemoveRequest identifies item rows rather than their parent orders.
type OrderLinesRemoveRequest struct {
	ItemIDs []int `json:"item_ids"`
}
