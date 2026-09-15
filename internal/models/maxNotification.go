package models

import "time"

type MaxNotificationSendRequest struct {
	MaxUserID int64          `json:"max_user_id"`
	Text      string         `json:"text"`
	Metadata  map[string]any `json:"metadata"`
}

type MaxNotificationSendResponse struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}
