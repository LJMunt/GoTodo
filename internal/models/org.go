package models

import "time"

type OrgRole string

const (
	RoleAdmin  OrgRole = "admin"
	RoleMember OrgRole = "member"
)

type Organization struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	WorkspacePublicID string     `json:"workspace_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
}

type OrgMember struct {
	OrgID        int64     `json:"org_id"`
	UserID       int64     `json:"user_id"`
	Role         OrgRole   `json:"role"`
	JoinedAt     time.Time `json:"joined_at"`
	UserPublicID string    `json:"user_public_id"`
	UserEmail    string    `json:"user_email"`
}
