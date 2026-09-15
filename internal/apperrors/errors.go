// Package apperrors
package apperrors

import "github.com/dronm/webapp"

const (
	CodeInvalidCredentials = "invalid_credentials"
	CodeUserBanned         = "user_banned"
	CodeUserDeviceBanned   = "user_device_banned"
	CodeSessionRequired    = "session_required"
	CodeForbidden          = "forbidden"
)

func InvalidCredentials() error {
	return webapp.Unauthorized("Неверное имя пользователя или пароль", map[string]any{
		"code": CodeInvalidCredentials,
	})
}

func UserBanned() error {
	return webapp.Unauthorized("Доступ для пользователя запрещен", map[string]any{
		"code": CodeUserBanned,
	})
}

func UserDeviceBanned() error {
	return webapp.Unauthorized("Вход с данного устройства запрещен", map[string]any{
		"code": CodeUserBanned,
	})
}

func SessionRequired() error {
	return webapp.Unauthorized("Необходима активная сессия", map[string]any{
		"code": CodeSessionRequired,
	})
}

func Forbidden(permissionCode string) error {
	return webapp.Forbidden("Недостаточно прав", map[string]any{
		"code":       CodeForbidden,
		"permission": permissionCode,
	});
}
