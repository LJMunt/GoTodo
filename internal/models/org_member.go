package models

import "time"

type OrgRole string

const (
	RoleAdmin  OrgRole = "admin"
	RoleMember OrgRole = "member"
)

type OrgMember struct {
	OrgID    int64     `json:"org_id"`
	UserID   int64     `json:"user_id"`
	Role     OrgRole   `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}
