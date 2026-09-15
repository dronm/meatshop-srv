package models

import (
	"time"

	wmodels "github.com/dronm/webapp/models"
)

const integration1CJobRelation = "integration_1c.jobs"

// Integration1CJob is a read-only public projection of an asynchronous 1C job.
type Integration1CJob struct {
	ID            int64      `json:"id" primaryKey:"true"`
	Command       string     `json:"command"`
	Params        JSONValue  `json:"params"`
	CorrelationID *string    `json:"correlation_id"`
	Metadata      JSONValue  `json:"metadata"`
	Status        string     `json:"status"`
	Priority      int        `json:"priority"`
	AvailableAt   time.Time  `json:"available_at"`
	AttemptCount  int        `json:"attempt_count"`
	MaxAttempts   int        `json:"max_attempts"`
	LockedAt      *time.Time `json:"locked_at"`
	LockedBy      *string    `json:"locked_by"`
	LastError     *string    `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

func (m Integration1CJob) Relation() string {
	return integration1CJobRelation
}

func (m Integration1CJob) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}
