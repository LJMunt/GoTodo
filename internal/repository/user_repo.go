package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	GetByPublicID(ctx context.Context, db DBTX, publicID string) (*models.User, error)
	GetByID(ctx context.Context, db DBTX, id int64) (*models.User, error)
	IsMemberOfOrg(ctx context.Context, db DBTX, orgID, userID int64) (bool, error)
	Update(ctx context.Context, db DBTX, user *models.User) error
	Delete(ctx context.Context, db DBTX, id int64) error
	Search(ctx context.Context, db DBTX, query string, limit int) ([]*models.User, error)
}

type userRepository struct{}

func NewUserRepository() UserRepository {
	return &userRepository{}
}

func (r *userRepository) GetByPublicID(ctx context.Context, db DBTX, publicID string) (*models.User, error) {
	var u models.User
	err := db.QueryRow(ctx,
		`SELECT id, public_id, email, password_hash, is_admin, is_active, email_verified_at, token_version, last_login, ui_theme, show_completed_default, language, totp_enabled, created_at, updated_at
		 FROM users WHERE public_id = $1`,
		publicID,
	).Scan(&u.ID, &u.PublicID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.EmailVerifiedAt, &u.TokenVersion, &u.LastLogin, &u.UITheme, &u.ShowCompletedDefault, &u.Language, &u.TOTPEnabled, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetByID(ctx context.Context, db DBTX, id int64) (*models.User, error) {
	var u models.User
	err := db.QueryRow(ctx,
		`SELECT id, public_id, email, password_hash, is_admin, is_active, email_verified_at, token_version, last_login, ui_theme, show_completed_default, language, totp_enabled, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.PublicID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.EmailVerifiedAt, &u.TokenVersion, &u.LastLogin, &u.UITheme, &u.ShowCompletedDefault, &u.Language, &u.TOTPEnabled, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) Update(ctx context.Context, db DBTX, u *models.User) error {
	_, err := db.Exec(ctx,
		`UPDATE users SET email=$1, password_hash=$2, is_admin=$3, is_active=$4, 
		                email_verified_at=$5, token_version=$6, last_login=$7, 
		                ui_theme=$8, show_completed_default=$9, language=$10, 
		                totp_enabled=$11, updated_at=now()
		 WHERE id=$12`,
		u.Email, u.PasswordHash, u.IsAdmin, u.IsActive,
		u.EmailVerifiedAt, u.TokenVersion, u.LastLogin,
		u.UITheme, u.ShowCompletedDefault, u.Language,
		u.TOTPEnabled, u.ID,
	)
	return err
}

func (r *userRepository) Delete(ctx context.Context, db DBTX, id int64) error {
	_, err := db.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
	return err
}

func (r *userRepository) Search(ctx context.Context, db DBTX, query string, limit int) ([]*models.User, error) {
	rows, err := db.Query(ctx,
		`SELECT id, public_id, email, password_hash, is_admin, is_active, email_verified_at, token_version, last_login, ui_theme, show_completed_default, language, totp_enabled, created_at, updated_at
		 FROM users
		 WHERE (email ILIKE $1 || '%' OR public_id ILIKE $1 || '%') AND is_active = true
		 LIMIT $2`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.PublicID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.EmailVerifiedAt, &u.TokenVersion, &u.LastLogin, &u.UITheme, &u.ShowCompletedDefault, &u.Language, &u.TOTPEnabled, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (r *userRepository) IsMemberOfOrg(ctx context.Context, db DBTX, orgID, userID int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2)",
		orgID, userID,
	).Scan(&ok)
	return ok, err
}
