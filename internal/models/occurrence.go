package models

import "time"

type Occurrence struct {
	ID              int64      `json:"id"`
	TaskID          int64      `json:"task_id"`
	OccurrenceIndex int64      `json:"occurrence_index"`
	DueAt           time.Time  `json:"due_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}
