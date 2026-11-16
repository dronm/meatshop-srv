package models

// OrderLinesWeightConfirmationRequest explicitly confirms or resets selected
// item weights. A pointer distinguishes false from an omitted JSON property.
type OrderLinesWeightConfirmationRequest struct {
	ItemIDs         []int `json:"item_ids"`
	WeightConfirmed *bool `json:"weight_confirmed"`
}
