package models

import "time"

type User struct {
	ID                   int64      `json:"id"`
	PublicID             string     `json:"public_id"`
	Email                string     `json:"email"`
	PasswordHash         string     `json:"-"`
	IsAdmin              bool       `json:"is_admin"`
	IsActive             bool       `json:"is_active"`
	EmailVerifiedAt      *time.Time `json:"email_verified_at"`
	TokenVersion         int64      `json:"-"`
	LastLogin            *time.Time `json:"last_login"`
	UITheme              string     `json:"ui_theme"`
	ShowCompletedDefault bool       `json:"show_completed_default"`
	Language             string     `json:"language"`
	TOTPEnabled          bool       `json:"totp_enabled"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
