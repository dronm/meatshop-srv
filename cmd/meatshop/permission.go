package main

import (
	"net/http"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/meatshop/internal/services"
	"github.com/dronm/webapp"
)

func NewPermissionChecker(permissionService *services.PermissionService) webapp.PermissionChecker {
	return func(r *http.Request, permission string) error {
		if permission == "" {
			return nil
		}

		sess := webapp.SessionFromContext(r.Context())
		if sess == nil {
			return webapp.Unauthorized("Session required", map[string]any{
				"code": "session_required",
			})
		}

		userLogin := models.UserLogin{}
		if err := sess.Get("user", &userLogin); err != nil {
			return webapp.Unauthorized("Session required", map[string]any{
				"code": "session_required",
			})
		}

		if !permissionService.Can(userLogin.RoleID, permission) {
			return webapp.Forbidden("Permission denied", map[string]any{
				"code":       "forbidden",
				"permission": permission,
			})
		}

		return nil
	}
}
