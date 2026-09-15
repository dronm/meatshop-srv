package models

import (
	"time"

	"github.com/dronm/modelbind"
	wmodels "github.com/dronm/webapp/models"
)

const mainMenuRelation = "public.main_menus"

// MainMenu is the writable menu item model.
// Exactly one of RoleID and UserID must be set.
type MainMenu struct {
	ID        int       `json:"id" primaryKey:"true" srvCalc:"true"`
	RoleID    *RoleID   `json:"role_id" enum:"role_id"`
	UserID    *int      `json:"user_id"`
	ParentID  *int      `json:"parent_id"`
	Caption   string    `json:"caption" required:"true" maxLen:"250"`
	RouteID   *int      `json:"route_id"`
	Icon      *string   `json:"icon,omitempty" maxLen:"250"`
	SortOrder int       `json:"sort_order"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at" srvCalc:"true"`
	UpdatedAt time.Time `json:"updated_at" srvCalc:"true"`
}

func (m MainMenu) Relation() string {
	return mainMenuRelation
}

func (m MainMenu) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}

// MainMenuKey is used for detail/update/delete WHERE clauses.
type MainMenuKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (m MainMenuKey) Relation() string {
	return mainMenuRelation
}

type UpdateMainMenuRequest struct {
	ID    int
	Input modelbind.ModelInput[*MainMenu]
}
