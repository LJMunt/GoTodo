package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrgService interface {
	CreateOrganization(ctx context.Context, userID int64, name string) (*models.Organization, error)
	ListOrganizations(ctx context.Context, userID int64) ([]*models.Organization, error)
	GetOrganization(ctx context.Context, userID, orgID int64) (*models.Organization, error)
	UpdateOrganization(ctx context.Context, userID, orgID int64, name string) (*models.Organization, error)
	DeleteOrganization(ctx context.Context, userID, orgID int64) error

	AddMember(ctx context.Context, requesterID, orgID int64, userPublicID string, role models.OrgRole) error
	RemoveMember(ctx context.Context, requesterID, orgID int64, userPublicID string) error
	LeaveOrganization(ctx context.Context, userID, orgID int64) error
	ListMembers(ctx context.Context, userID, orgID int64) ([]*models.OrgMember, error)
	GetMemberRole(ctx context.Context, userID, orgID int64) (models.OrgRole, error)
}

type orgService struct {
	pool     *pgxpool.Pool
	orgRepo  repository.OrganizationRepository
	userRepo repository.UserRepository
}

func NewOrgService(pool *pgxpool.Pool, orgRepo repository.OrganizationRepository, userRepo repository.UserRepository) OrgService {
	return &orgService{
		pool:     pool,
		orgRepo:  orgRepo,
		userRepo: userRepo,
	}
}

func (s *orgService) CreateOrganization(ctx context.Context, userID int64, name string) (*models.Organization, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	org, err := s.orgRepo.Create(ctx, tx, name)
	if err != nil {
		return nil, err
	}

	// Creator becomes admin
	if err := s.orgRepo.AddMember(ctx, tx, org.ID, userID, models.RoleAdmin); err != nil {
		return nil, err
	}

	// Create workspace for org
	_, err = tx.Exec(ctx,
		`INSERT INTO workspaces (public_id, type, org_id) VALUES ($1, 'org', $2)`,
		"org_"+org.Name, // This is just a placeholder, should use proper ID generation
		org.ID,
	)
	// TODO: use proper public_id generation (26 chars)

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return org, nil
}

func (s *orgService) ListOrganizations(ctx context.Context, userID int64) ([]*models.Organization, error) {
	return s.orgRepo.ListByUserID(ctx, s.pool, userID)
}

func (s *orgService) GetOrganization(ctx context.Context, userID, orgID int64) (*models.Organization, error) {
	role, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, userID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}

	return s.orgRepo.GetByID(ctx, s.pool, orgID)
}

func (s *orgService) UpdateOrganization(ctx context.Context, userID, orgID int64, name string) (*models.Organization, error) {
	role, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, userID)
	if err != nil {
		return nil, err
	}
	if role != models.RoleAdmin {
		return nil, ErrForbidden
	}

	org, err := s.orgRepo.GetByID(ctx, s.pool, orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrNotFound
	}

	org.Name = name
	if err := s.orgRepo.Update(ctx, s.pool, org); err != nil {
		return nil, err
	}

	return org, nil
}

func (s *orgService) DeleteOrganization(ctx context.Context, userID, orgID int64) error {
	role, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, userID)
	if err != nil {
		return err
	}
	if role != models.RoleAdmin {
		return ErrForbidden
	}

	return s.orgRepo.Delete(ctx, s.pool, orgID)
}

func (s *orgService) AddMember(ctx context.Context, requesterID, orgID int64, userPublicID string, role models.OrgRole) error {
	reqRole, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, requesterID)
	if err != nil {
		return err
	}
	if reqRole != models.RoleAdmin {
		return ErrForbidden
	}

	user, err := s.userRepo.GetByPublicID(ctx, s.pool, userPublicID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("%w: user not found", ErrInvalidInput)
	}

	return s.orgRepo.AddMember(ctx, s.pool, orgID, user.ID, role)
}

func (s *orgService) RemoveMember(ctx context.Context, requesterID, orgID int64, userPublicID string) error {
	reqRole, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, requesterID)
	if err != nil {
		return err
	}
	if reqRole != models.RoleAdmin {
		return ErrForbidden
	}

	user, err := s.userRepo.GetByPublicID(ctx, s.pool, userPublicID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("%w: user not found", ErrInvalidInput)
	}

	if user.ID == requesterID {
		return fmt.Errorf("%w: cannot remove yourself, use leave instead", ErrInvalidInput)
	}

	return s.orgRepo.RemoveMember(ctx, s.pool, orgID, user.ID)
}

func (s *orgService) LeaveOrganization(ctx context.Context, userID, orgID int64) error {
	role, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, userID)
	if err != nil {
		return err
	}
	if role == "" {
		return ErrForbidden
	}

	if role == models.RoleAdmin {
		// Check if there are other admins
		members, err := s.orgRepo.ListMembers(ctx, s.pool, orgID)
		if err != nil {
			return err
		}
		adminCount := 0
		for _, m := range members {
			if m.Role == models.RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return fmt.Errorf("%w: cannot leave as last admin, delete org or promote someone else", ErrInvalidInput)
		}
	}

	return s.orgRepo.RemoveMember(ctx, s.pool, orgID, userID)
}

func (s *orgService) ListMembers(ctx context.Context, userID, orgID int64) ([]*models.OrgMember, error) {
	role, err := s.orgRepo.GetMemberRole(ctx, s.pool, orgID, userID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}

	return s.orgRepo.ListMembers(ctx, s.pool, orgID)
}

func (s *orgService) GetMemberRole(ctx context.Context, userID, orgID int64) (models.OrgRole, error) {
	return s.orgRepo.GetMemberRole(ctx, s.pool, orgID, userID)
}
