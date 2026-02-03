package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	authmw "GoToDo/internal/auth"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordChangeHandler_NotAuthenticated(t *testing.T) {
	db := fakeAuthDB{}

	body := `{"currentPassword":"OldPassword123!","newPassword":"NewPassword456!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-change", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	PasswordChangeHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	expectedError := "not authenticated"
	if !bytes.Contains(rec.Body.Bytes(), []byte(expectedError)) {
		t.Fatalf("expected error message %q, got %q", expectedError, rec.Body.String())
	}
}

func TestPasswordChangeHandler_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	oldPassword := "OldPassword123!"
	newPassword := "NewPassword456!"
	userID := int64(42)

	hash, _ := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			if args[0] != userID {
				t.Errorf("expected user id %d, got %v", userID, args[0])
			}
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = string(hash)
					return nil
				},
			}
		},
	}

	body := `{"currentPassword":"` + oldPassword + `","newPassword":"` + newPassword + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-change", bytes.NewBufferString(body))

	// Simulate middleware
	ctx := authmw.WithUser(req.Context(), authmw.User{ID: userID})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	PasswordChangeHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}
