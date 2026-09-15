package models

import (
	"time"

	"github.com/dronm/modelbind"
)

type ObjectHistoryQuery struct {
	ObjectType string `json:"object_type" required:"true"`
	ObjectID   string `json:"object_id" required:"true"`
}

type ObjectHistoryInput struct {
	Query  *ObjectHistoryQuery
	Params modelbind.CollectionParams
}

type ObjectHistoryChange struct {
	Column         string  `json:"column"`
	Field          string  `json:"field"`
	Old            any     `json:"old"`
	New            any     `json:"new"`
	OldDescription *string `json:"old_description,omitempty"`
	NewDescription *string `json:"new_description,omitempty"`
}

type ObjectHistoryRow struct {
	ID        int64                  `json:"id"`
	ChangedAt time.Time              `json:"changed_at"`
	Operation string                 `json:"operation"`
	ChangedBy *string                `json:"changed_by"`
	Changes   []*ObjectHistoryChange `json:"changes"`
}

type ObjectHistoryResponse struct {
	Rows  []*ObjectHistoryRow `json:"rows"`
	Total int64               `json:"total"`
}
