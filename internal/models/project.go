package models

import "time"

type Project struct {
	ID          int64      `json:"id"`
	WorkspaceID int64      `json:"workspace_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
