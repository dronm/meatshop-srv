package models

import wmodels "github.com/dronm/webapp/models"

const customerUserListRelation = "public.customer_users_list"

// CustomerUserList is a deduplicated MAX user that has placed at least one
// order for a customer. CustomerID is intentionally exposed so collection
// filters can scope the list to the customer being edited.
type CustomerUserList struct {
	CustomerID int    `json:"customer_id"`
	ID         int    `json:"id" primaryKey:"true"`
	MaxUserID  int64  `json:"max_user_id"`
	Username   string `json:"username"`
	AvatarURL  *string `json:"avatar_url"`
	IsActive   bool   `json:"is_active"`
}

func (m CustomerUserList) Relation() string {
	return customerUserListRelation
}

func (m CustomerUserList) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}
