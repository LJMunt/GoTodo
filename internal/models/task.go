package models

import "time"

type Task struct {
	ID                int64      `json:"id"`
	WorkspaceID       int64      `json:"-"`
	ProjectID         int64      `json:"project_id"`
	Title             string     `json:"title"`
	Description       *string    `json:"description,omitempty"`
	DueAt             *time.Time `json:"due_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
	RepeatEvery       *int       `json:"repeat_every,omitempty"`
	RepeatUnit        *string    `json:"repeat_unit,omitempty"`
	RecurrenceStartAt *time.Time `json:"recurrence_start_at,omitempty"`
	NextDueAt         *time.Time `json:"next_due_at,omitempty"`
	CreatedBy         int64      `json:"-"`
	ClosedBy          *int64     `json:"-"`
	AssignedTo        *int64     `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
