package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/meatshop/internal/services"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	"github.com/dronm/webapp/ws"
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

// The WebSocket bridge also exposes service methods directly. Apply the same
// inbox permissions as HTTP without altering existing services' behavior.
func customerMessageWSChecker(check webapp.PermissionChecker) ws.PermissionChecker {
	return func(sess session.Session, method string) error {
		if !strings.HasPrefix(method, "CustomerMessage.") {
			return nil
		}
		permissions := map[string]string{
			"CustomerMessage.List":        "customerMessage.list",
			"CustomerMessage.Detail":      "customerMessage.detail",
			"CustomerMessage.UnreadCount": "customerMessage.list",
			"CustomerMessage.MarkRead":    "customerMessage.markRead",
		}
		permission, ok := permissions[method]
		if !ok || check == nil {
			return webapp.Forbidden("Customer message method is not allowed", nil)
		}
		request := (&http.Request{}).WithContext(webapp.ContextWithSession(context.Background(), sess))
		return check(request, permission)
	}
}
