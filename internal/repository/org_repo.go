package repository

import (
	"GoToDo/internal/models"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type OrganizationRepository interface {
	Create(ctx context.Context, db DBTX, name string) (*models.Organization, error)
	GetByID(ctx context.Context, db DBTX, id int64) (*models.Organization, error)
	Update(ctx context.Context, db DBTX, org *models.Organization) error
	Delete(ctx context.Context, db DBTX, id int64) error
	ListByUserID(ctx context.Context, db DBTX, userID int64) ([]*models.Organization, error)

	AddMember(ctx context.Context, db DBTX, orgID, userID int64, role models.OrgRole) error
	RemoveMember(ctx context.Context, db DBTX, orgID, userID int64) error
	GetMemberRole(ctx context.Context, db DBTX, orgID, userID int64) (models.OrgRole, error)
	ListMembers(ctx context.Context, db DBTX, orgID int64) ([]*models.OrgMember, error)
}

type organizationRepository struct{}

func NewOrganizationRepository() OrganizationRepository {
	return &organizationRepository{}
}

func (r *organizationRepository) Create(ctx context.Context, db DBTX, name string) (*models.Organization, error) {
	var o models.Organization
	err := db.QueryRow(ctx,
		`INSERT INTO orgs (name) VALUES ($1) RETURNING id, name, created_at, updated_at`,
		name,
	).Scan(&o.ID, &o.Name, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *organizationRepository) GetByID(ctx context.Context, db DBTX, id int64) (*models.Organization, error) {
	var o models.Organization
	err := db.QueryRow(ctx,
		`SELECT id, name, created_at, updated_at, deleted_at FROM orgs WHERE id=$1 AND deleted_at IS NULL`,
		id,
	).Scan(&o.ID, &o.Name, &o.CreatedAt, &o.UpdatedAt, &o.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func (r *organizationRepository) Update(ctx context.Context, db DBTX, o *models.Organization) error {
	_, err := db.Exec(ctx,
		`UPDATE orgs SET name=$1, updated_at=now() WHERE id=$2`,
		o.Name, o.ID,
	)
	return err
}

func (r *organizationRepository) Delete(ctx context.Context, db DBTX, id int64) error {
	_, err := db.Exec(ctx, "UPDATE orgs SET deleted_at=now(), updated_at=now() WHERE id=$1", id)
	return err
}

func (r *organizationRepository) ListByUserID(ctx context.Context, db DBTX, userID int64) ([]*models.Organization, error) {
	rows, err := db.Query(ctx,
		`SELECT o.id, o.name, o.created_at, o.updated_at
		 FROM orgs o
		 JOIN org_members om ON o.id = om.org_id
		 WHERE om.user_id = $1 AND o.deleted_at IS NULL
		 ORDER BY o.name ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*models.Organization
	for rows.Next() {
		var o models.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, &o)
	}
	return orgs, nil
}

func (r *organizationRepository) AddMember(ctx context.Context, db DBTX, orgID, userID int64, role models.OrgRole) error {
	_, err := db.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)`,
		orgID, userID, role,
	)
	return err
}

func (r *organizationRepository) RemoveMember(ctx context.Context, db DBTX, orgID, userID int64) error {
	_, err := db.Exec(ctx,
		`DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	return err
}

func (r *organizationRepository) GetMemberRole(ctx context.Context, db DBTX, orgID, userID int64) (models.OrgRole, error) {
	var role models.OrgRole
	err := db.QueryRow(ctx,
		`SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

func (r *organizationRepository) ListMembers(ctx context.Context, db DBTX, orgID int64) ([]*models.OrgMember, error) {
	rows, err := db.Query(ctx,
		`SELECT om.org_id, om.user_id, om.role, om.joined_at, u.public_id, u.email
		 FROM org_members om
		 JOIN users u ON u.id = om.user_id
		 WHERE om.org_id = $1
		 ORDER BY om.joined_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.OrgMember
	for rows.Next() {
		var m models.OrgMember
		if err := rows.Scan(&m.OrgID, &m.UserID, &m.Role, &m.JoinedAt, &m.UserPublicID, &m.UserEmail); err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	return members, nil
}
