package service

import (
	"GoToDo/internal/models"
	"GoToDo/internal/repository"
	"context"
	"testing"
)

type mockOrgRepo struct {
	repository.OrganizationRepository
	GetMemberRoleFunc func(ctx context.Context, db repository.DBTX, orgID, userID int64) (models.OrgRole, error)
}

func (m *mockOrgRepo) GetMemberRole(ctx context.Context, db repository.DBTX, orgID, userID int64) (models.OrgRole, error) {
	return m.GetMemberRoleFunc(ctx, db, orgID, userID)
}

func TestOrgAccess(t *testing.T) {
	repo := &mockOrgRepo{
		GetMemberRoleFunc: func(ctx context.Context, db repository.DBTX, orgID, userID int64) (models.OrgRole, error) {
			if userID == 1 {
				return models.RoleAdmin, nil
			}
			if userID == 2 {
				return models.RoleMember, nil
			}
			return "", nil
		},
	}
	svc := NewOrgService(nil, repo, nil)

	t.Run("AdminAccess", func(t *testing.T) {
		role, err := svc.GetMemberRole(context.Background(), 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		if role != models.RoleAdmin {
			t.Errorf("expected admin role, got %v", role)
		}
	})

	t.Run("MemberAccess", func(t *testing.T) {
		role, err := svc.GetMemberRole(context.Background(), 2, 100)
		if err != nil {
			t.Fatal(err)
		}
		if role != models.RoleMember {
			t.Errorf("expected member role, got %v", role)
		}
	})

	t.Run("NoAccess", func(t *testing.T) {
		role, err := svc.GetMemberRole(context.Background(), 3, 100)
		if err != nil {
			t.Fatal(err)
		}
		if role != "" {
			t.Errorf("expected no role, got %v", role)
		}
	})
}
