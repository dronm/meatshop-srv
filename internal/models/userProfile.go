package models

// UserProfile is the common self-service profile returned to the currently
// authenticated user. Role information is intentionally excluded: changing a
// role is an administrative operation, not a profile operation.
type UserProfile struct {
	ID   int    `json:"id" primaryKey:"true"`
	Name string `json:"name" required:"true" maxLen:"100"`
}

func (m UserProfile) Relation() string {
	return userRelation
}

// UserProfileUpdate contains fields the current user may change for itself.
// Applications with additional profile attributes can extend their own
// backend model while keeping the same /users/me endpoint contract.
type UserProfileUpdate struct {
	Name string `json:"name" required:"true" maxLen:"100"`
}

// UserPasswordChange is intentionally self-scoped by the /users/me/password
// route. There is no user ID in this request, so callers cannot target another
// user's password.
type UserPasswordChange struct {
	NewPassword string `json:"new_password" required:"true" maxLen:"50"`
}
