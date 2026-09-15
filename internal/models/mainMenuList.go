package models

import (
	"time"

	wmodels "github.com/dronm/webapp/models"
)

const mainMenuListRelation = "public.main_menus_list"

// MainMenuList is the collection view used by the menu-management UI.
type MainMenuList struct {
	ID               int       `json:"id" primaryKey:"true" srvCalc:"true"`
	RoleID           *RoleID   `json:"role_id" enum:"role_id"`
	User             *Ref      `json:"users_ref"`
	Parent           *Ref      `json:"main_menus_ref"`
	ApplicationRoute *Ref      `json:"application_routes_ref"`
	RouteName        *string   `json:"route_name,omitempty"`
	Caption          string    `json:"caption" required:"true" maxLen:"250"`
	Icon             *string   `json:"icon,omitempty" maxLen:"250"`
	SortOrder        int       `json:"sort_order"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (m MainMenuList) Relation() string {
	return mainMenuListRelation
}

func (m MainMenuList) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}
