package maxbot

import (
	"encoding/json"
	"time"
)

type User struct {
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name"`
	Name      string  `json:"name"`
	Username  *string `json:"username"`
}

type Recipient struct {
	ChatID *int64 `json:"chat_id"`
	UserID *int64 `json:"user_id"`
}

type Message struct {
	Sender    *User      `json:"sender"`
	Recipient *Recipient `json:"recipient"`
}

type Update struct {
	UpdateType string          `json:"update_type"`
	ChatID     *int64          `json:"chat_id"`
	User       *User           `json:"user"`
	Message    *Message        `json:"message"`
	RawUser    json.RawMessage `json:"-"`
}

type OutMessage struct {
	ID           int64
	MaxUserID    int64
	Message      json.RawMessage
	AttemptCount int
}

type SenderConfig struct {
	DSN                     string
	PollInterval            time.Duration
	NotifyReconnectInterval time.Duration
	LockTimeout             time.Duration
	RetryBaseDelay          time.Duration
	RetryMaxDelay           time.Duration
	MaxAttempts             int
}
