package orgs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type OrganizationResponse struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	WorkspacePublicID string     `json:"workspace_id"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type MemberResponse struct {
	PublicID string    `json:"public_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func parseInt64Param(r *http.Request, key string) (int64, error) {
	s := chi.URLParam(r, key)
	return strconv.ParseInt(s, 10, 64)
}

type orgDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

func CreateOrganizationHandler(db orgDB) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		tx, err := db.Begin(ctx)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to start transaction")
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var orgID int64
		var createdAt, updatedAt time.Time
		err = tx.QueryRow(ctx,
			`INSERT INTO orgs (name) VALUES ($1) RETURNING id, created_at, updated_at`,
			req.Name,
		).Scan(&orgID, &createdAt, &updatedAt)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to create organization")
			return
		}

		workspacePublicID := app.NewULID()
		_, err = tx.Exec(ctx,
			`INSERT INTO workspaces (type, org_id, public_id) VALUES ('org', $1, $2)`,
			orgID, workspacePublicID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to create organization workspace")
			return
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, 'admin')`,
			orgID, user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to add creator as admin")
			return
		}

		if err := tx.Commit(ctx); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
			return
		}

		writeJSON(w, http.StatusCreated, OrganizationResponse{
			ID:                orgID,
			Name:              req.Name,
			WorkspacePublicID: workspacePublicID,
			CreatedAt:         createdAt,
			UpdatedAt:         updatedAt,
		})
	}
}

func ListOrganizationsHandler(db orgDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		rows, err := db.Query(ctx,
			`SELECT o.id, o.name, w.public_id, o.deleted_at, o.created_at, o.updated_at
			 FROM orgs o
			 JOIN workspaces w ON w.org_id = o.id
			 JOIN org_members om ON om.org_id = o.id
			 WHERE om.user_id = $1 AND o.deleted_at IS NULL`,
			user.ID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list organizations")
			return
		}
		defer rows.Close()

		var orgs []OrganizationResponse
		for rows.Next() {
			var o OrganizationResponse
			if err := rows.Scan(&o.ID, &o.Name, &o.WorkspacePublicID, &o.DeletedAt, &o.CreatedAt, &o.UpdatedAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read organizations")
				return
			}
			orgs = append(orgs, o)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read organizations")
			return
		}

		writeJSON(w, http.StatusOK, orgs)
	}
}

func UpdateOrganizationHandler(db orgDB) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`UPDATE orgs SET name = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`,
			req.Name, orgID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update organization")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "organization not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteOrganizationHandler(db orgDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		tag, err := db.Exec(ctx,
			`UPDATE orgs SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
			orgID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to delete organization")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "organization not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func AddMemberHandler(db orgDB) http.HandlerFunc {
	type request struct {
		PublicID string `json:"public_id"`
		Role     string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.PublicID == "" {
			writeErr(w, http.StatusBadRequest, "public_id is required")
			return
		}
		if req.Role == "" {
			req.Role = "member"
		}
		if req.Role != "admin" && req.Role != "member" {
			writeErr(w, http.StatusBadRequest, "invalid role")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var internalUserID int64
		err = db.QueryRow(ctx, `SELECT id FROM users WHERE public_id = $1`, req.PublicID).Scan(&internalUserID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to find user")
			return
		}

		_, err = db.Exec(ctx,
			`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)
			 ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
			orgID, internalUserID, req.Role,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to add member")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func RemoveMemberHandler(db orgDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		userPublicID := chi.URLParam(r, "userId")
		if userPublicID == "" {
			writeErr(w, http.StatusBadRequest, "public_id is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var internalUserID int64
		err = db.QueryRow(ctx, `SELECT id FROM users WHERE public_id = $1`, userPublicID).Scan(&internalUserID)
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to find user")
			return
		}

		// Harden: Check if user is the last admin
		var role string
		err = db.QueryRow(ctx, "SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2", orgID, internalUserID).Scan(&role)
		if err != nil {
			writeErr(w, http.StatusNotFound, "member not found")
			return
		}

		if role == "admin" {
			var adminCount int
			err = db.QueryRow(ctx, "SELECT count(*) FROM org_members WHERE org_id = $1 AND role = 'admin'", orgID).Scan(&adminCount)
			if err == nil && adminCount <= 1 {
				writeErr(w, http.StatusBadRequest, "cannot remove the last admin. promote another admin first or delete the organization.")
				return
			}
		}

		tag, err := db.Exec(ctx, `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, internalUserID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to remove member")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "member not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func GetOrganizationHandler(db orgDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var o OrganizationResponse
		err = db.QueryRow(ctx,
			`SELECT o.id, o.name, w.public_id, o.deleted_at, o.created_at, o.updated_at
			 FROM orgs o
			 JOIN workspaces w ON w.org_id = o.id
			 WHERE o.id = $1 AND o.deleted_at IS NULL`,
			orgID,
		).Scan(&o.ID, &o.Name, &o.WorkspacePublicID, &o.DeletedAt, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusNotFound, "organization not found")
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to read organization")
			return
		}

		writeJSON(w, http.StatusOK, o)
	}
}

func LeaveOrganizationHandler(db orgDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Check if user is the last admin
		var role string
		err = db.QueryRow(ctx, "SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2", orgID, user.ID).Scan(&role)
		if err != nil {
			writeErr(w, http.StatusNotFound, "not a member of this organization")
			return
		}

		if role == "admin" {
			var adminCount int
			err = db.QueryRow(ctx, "SELECT count(*) FROM org_members WHERE org_id = $1 AND role = 'admin'", orgID).Scan(&adminCount)
			if err == nil && adminCount <= 1 {
				writeErr(w, http.StatusBadRequest, "cannot leave organization as the last admin. delete the organization instead.")
				return
			}
		}

		tag, err := db.Exec(ctx, `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, user.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to leave organization")
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "not a member of this organization")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func ListMembersHandler(db orgDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseInt64Param(r, "id")
		if err != nil || orgID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		rows, err := db.Query(ctx,
			`SELECT u.public_id, u.email, om.role, om.joined_at
			 FROM org_members om
			 JOIN users u ON u.id = om.user_id
			 WHERE om.org_id = $1
			 ORDER BY om.joined_at ASC`,
			orgID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to list members")
			return
		}
		defer rows.Close()

		var members []MemberResponse
		for rows.Next() {
			var m MemberResponse
			if err := rows.Scan(&m.PublicID, &m.Email, &m.Role, &m.JoinedAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read members")
				return
			}
			members = append(members, m)
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read members")
			return
		}

		writeJSON(w, http.StatusOK, members)
	}
}
