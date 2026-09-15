package models

import (
	"time"

	wmodels "github.com/dronm/webapp/models"
)

const applicationRouteRelation = "public.application_routes"

// ApplicationRoute is a route registered by the compiled frontend manifest.
// Technical fields are synchronized from the manifest, while Descr, Section,
// and Icon may be adjusted by an administrator.
type ApplicationRoute struct {
	ID            int       `json:"id" primaryKey:"true" srvCalc:"true"`
	Name          string    `json:"name" srvCalc:"true"`
	Path          string    `json:"path" srvCalc:"true"`
	Descr         string    `json:"descr" required:"true" maxLen:"250"`
	Section       string    `json:"section" required:"true" maxLen:"250"`
	Icon          *string   `json:"icon,omitempty" maxLen:"250"`
	MenuAvailable bool      `json:"menu_available" srvCalc:"true"`
	IsActive      bool      `json:"is_active" srvCalc:"true"`
	CreatedAt     time.Time `json:"created_at" srvCalc:"true"`
	UpdatedAt     time.Time `json:"updated_at" srvCalc:"true"`
}

func (m ApplicationRoute) Relation() string {
	return applicationRouteRelation
}

func (m ApplicationRoute) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}

type ApplicationRouteKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (m ApplicationRouteKey) Relation() string {
	return applicationRouteRelation
}

type ApplicationRouteManifestItem struct {
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	Descr         string  `json:"descr"`
	Section       string  `json:"section"`
	Icon          *string `json:"icon"`
	MenuAvailable bool    `json:"menu_available"`
}

type ApplicationRouteSyncInput struct {
	Items []ApplicationRouteManifestItem `json:"items"`
}

type ApplicationRouteSyncResult struct {
	Registered  int `json:"registered"`
	Deactivated int `json:"deactivated"`
}
