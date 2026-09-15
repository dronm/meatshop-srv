package services

import (
	"fmt"

	"github.com/dronm/session"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
)

type Authorizer struct {
	Permissions *PermissionService
}

func NewAuthorizer(permissions *PermissionService) *Authorizer {
	return &Authorizer{
		Permissions: permissions,
	}
}

func (a *Authorizer) Require(sess session.Session, permissionCode string) (models.UserLogin, error) {
	if sess == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := sess.Get("user", &user); err != nil {
		return models.UserLogin{}, fmt.Errorf("session user: %w", apperrors.SessionRequired())
	}

	if a.Permissions == nil {
		return models.UserLogin{}, fmt.Errorf("permission service is nil")
	}

	if !a.Permissions.Can(user.RoleID, permissionCode) {
		return models.UserLogin{}, apperrors.Forbidden(permissionCode)
	}

	return user, nil
}
