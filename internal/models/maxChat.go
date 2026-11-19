package models

import "time"

type MaxChatParticipantsRequest struct {
	MessageID  string `json:"message_id"`
	Query      string `json:"q"`
	CustomerID int    `json:"customer_id"`
	UserID     int    `json:"user_id"`
	MaxUserID  string `json:"max_user_id"`
	OrderID    int    `json:"order_id"`
	Cursor     string `json:"cursor"`
	Limit      int    `json:"limit"`
}

type MaxChatParticipant struct {
	CustomerID   int     `json:"customer_id"`
	CustomerName string  `json:"customer_name"`
	UserID       int     `json:"user_id"`
	MaxUserID    string  `json:"max_user_id"`
	UserName     string  `json:"user_name"`
	Username     *string `json:"username"`
	IsActive     bool    `json:"is_active"`
}

type MaxChatParticipantPage struct {
	Rows       []MaxChatParticipant `json:"rows"`
	NextCursor *string              `json:"next_cursor"`
}

type MaxChatHistoryRequest struct {
	CustomerID int    `json:"customer_id"`
	UserID     int    `json:"user_id"`
	Before     string `json:"before"`
	After      string `json:"after"`
	IncomingID string `json:"incoming_id"`
	OutgoingID string `json:"outgoing_id"`
	Limit      int    `json:"limit"`
}

type MaxChatMessage struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	MessageAt  string     `json:"message_at"`
	Cursor     string     `json:"cursor"`
	Text       string     `json:"text"`
	AuthorName *string    `json:"author_name"`
	Status     *string    `json:"status"`
	SentAt     *time.Time `json:"sent_at"`
	ErrorStr   *string    `json:"error_str"`
	IsRead     bool       `json:"is_read"`
}

type MaxChatHistoryPage struct {
	Rows    []MaxChatMessage `json:"rows"`
	HasMore bool             `json:"has_more"`
}

type MaxChatSendRequest struct {
	OrderID    int    `json:"order_id,omitempty"`
	CustomerID int    `json:"customer_id"`
	UserID     int    `json:"user_id"`
	Text       string `json:"text"`
	RequestID  string `json:"request_id"`
}
