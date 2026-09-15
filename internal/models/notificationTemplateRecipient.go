package models

import (
	"github.com/dronm/modelbind"
	wmodels "github.com/dronm/webapp/models"
)

const notificationTemplateRecipientRelation = "public.notification_template_recipients"
const notificationTemplateRecipientListRelation = "public.notification_template_recipients_list"
const notificationTemplateRecipientDetailRelation = "public.notification_template_recipients_detail"

// NotificationTemplateRecipient binds a notification template to an internal employee.
// The employee's users.max_user_id reference is resolved when the notification is queued.
type NotificationTemplateRecipient struct {
	ID         int `json:"id" primaryKey:"true" srvCalc:"true"`
	TemplateID int `json:"template_id" required:"true"`
	UserID     int `json:"user_id" required:"true"`
}

func (m NotificationTemplateRecipient) Relation() string {
	return notificationTemplateRecipientRelation
}

func (m NotificationTemplateRecipient) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}

type NotificationTemplateRecipientList struct {
	ID         int  `json:"id" primaryKey:"true"`
	TemplateID int  `json:"template_id"`
	Template   *Ref `json:"template"`
	UserID     int  `json:"user_id"`
	User       *Ref `json:"user"`
}

func (m NotificationTemplateRecipientList) Relation() string {
	return notificationTemplateRecipientListRelation
}

func (m NotificationTemplateRecipientList) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}

// NotificationTemplateRecipientDetail contains reference projections used by edit forms.
type NotificationTemplateRecipientDetail struct {
	ID         int  `json:"id" primaryKey:"true"`
	TemplateID int  `json:"template_id"`
	Template   *Ref `json:"template"`
	UserID     int  `json:"user_id"`
	User       *Ref `json:"user"`
}

func (m NotificationTemplateRecipientDetail) Relation() string {
	return notificationTemplateRecipientDetailRelation
}

type NotificationTemplateRecipientKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (m NotificationTemplateRecipientKey) Relation() string {
	return notificationTemplateRecipientRelation
}

type UpdateNotificationTemplateRecipientRequest struct {
	ID    int
	Input modelbind.ModelInput[*NotificationTemplateRecipient]
}
