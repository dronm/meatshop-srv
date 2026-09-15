package models

import (
	"github.com/dronm/modelbind"
	wmodels "github.com/dronm/webapp/models"
)

const notificationTemplateRelation = "public.notification_templates"

// NotificationTemplate is an editable message template selected by a domain event.
type NotificationTemplate struct {
	ID           int    `json:"id" primaryKey:"true" srvCalc:"true"`
	Code         string `json:"code" required:"true" maxLen:"100"`
	Event        string `json:"event" required:"true" maxLen:"100"`
	BodyTemplate string `json:"body_template" required:"true"`
	IsActive     bool   `json:"is_active" required:"true"`
}

func (m NotificationTemplate) Relation() string {
	return notificationTemplateRelation
}

func (m NotificationTemplate) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}

type NotificationTemplateKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (m NotificationTemplateKey) Relation() string {
	return notificationTemplateRelation
}

type UpdateNotificationTemplateRequest struct {
	ID    int
	Input modelbind.ModelInput[*NotificationTemplate]
}
