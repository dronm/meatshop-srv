package services

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

type PermissionMap map[models.RoleID]map[string]struct{}

type PermissionService struct {
	DB    ds.Provider
	value atomic.Value
}

func NewPermissionService(db ds.Provider) *PermissionService {
	s := &PermissionService{
		DB: db,
	}

	s.value.Store(PermissionMap{})

	return s
}

func (s *PermissionService) Load(ctx context.Context) error {
	if s.DB == nil {
		return fmt.Errorf("db is nil")
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return err
	}
	defer s.DB.Release(poolConn, connID)

	rows, err := poolConn.Conn().Query(ctx, `
		SELECT
			role_id,
			permission_code
		FROM role_permissions
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	result := PermissionMap{}

	for rows.Next() {
		var roleID string
		var permissionCode string

		if err := rows.Scan(&roleID, &permissionCode); err != nil {
			return err
		}

		typedRoleID := models.RoleID(roleID)
		if _, ok := result[typedRoleID]; !ok {
			result[typedRoleID] = map[string]struct{}{}
		}

		result[typedRoleID][permissionCode] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	s.value.Store(result)

	return nil
}

func (s *PermissionService) Can(roleID models.RoleID, permissionCode string) bool {
	if permissionCode == "" {
		return true
	}

	raw := s.value.Load()
	if raw == nil {
		return false
	}

	permissions, ok := raw.(PermissionMap)
	if !ok {
		return false
	}

	rolePermissions, ok := permissions[roleID]
	if !ok {
		return false
	}

	_, ok = rolePermissions[permissionCode]

	return ok
}
