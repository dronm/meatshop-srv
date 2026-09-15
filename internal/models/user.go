package models

import "github.com/dronm/modelbind"

const userRelation = "public.users"

// User is the writable model used when creating a user.
// Password is accepted only during creation and is stored as an MD5 hash by
// UserService to remain compatible with the existing login query/database.
type User struct {
	ID        int    `json:"id" primaryKey:"true" srvCalc:"true"`
	Name      string `json:"name" required:"true" maxLen:"100"`
	RoleID    RoleID `json:"role_id" enum:"role_id"`
	Pwd       string `json:"pwd" required:"true" maxLen:"50"`
	MaxUserID *int   `json:"max_user_id"`
}

func (m User) Relation() string {
	return userRelation
}

// UserUpdate deliberately contains only fields that may be changed from the
// user detail dialog. Password changes should use a dedicated endpoint if they
// are introduced later.
type UserUpdate struct {
	Name      string `json:"name" required:"true" maxLen:"100"`
	RoleID    RoleID `json:"role_id" enum:"role_id"`
	MaxUserID *int   `json:"max_user_id"`
}

func (m UserUpdate) Relation() string {
	return userRelation
}

// UserKey is used for detail/update/delete WHERE clauses.
type UserKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (m UserKey) Relation() string {
	return userRelation
}

type UpdateUserRequest struct {
	ID    int
	Input modelbind.ModelInput[*UserUpdate]
}
