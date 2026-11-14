package models

type CustomerMessageMarkReadRequest struct {
	MessageIDs []int64 `json:"message_ids"`
}

type CustomerMessageUnreadCount struct {
	UnreadCount int64 `json:"unread_count"`
}
