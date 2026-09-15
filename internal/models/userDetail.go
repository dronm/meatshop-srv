package models

import wmodels "github.com/dronm/webapp/models"

const userDetailRelation = "public.users_detail"

// UserDetail is also the safe collection model. It intentionally excludes pwd
// so password hashes can never be returned by the user collection/detail API.
type UserDetail struct {
	ID      int    `json:"id" primaryKey:"true"`
	Name    string `json:"name" required:"true" maxLen:"100"`
	RoleID  RoleID `json:"role_id" enum:"role_id"`
	MaxUser *Ref   `json:"max_user"`
}

func (m UserDetail) Relation() string {
	return userDetailRelation
}

func (m UserDetail) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}
