package models

import "time"

type WorkspaceType string

const (
	WorkspaceTypeUser WorkspaceType = "user"
	WorkspaceTypeOrg  WorkspaceType = "org"
)

type Workspace struct {
	ID        int64         `json:"id"`
	PublicID  string        `json:"public_id"`
	Type      WorkspaceType `json:"type"`
	UserID    *int64        `json:"user_id,omitempty"`
	OrgID     *int64        `json:"org_id,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}
