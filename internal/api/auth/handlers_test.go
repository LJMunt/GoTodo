package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeAuthDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (db fakeAuthDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func (db fakeAuthDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func TestSignupHandler_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 42
					return nil
				},
			}
		},
	}

	body := `{"email":" Test@Email.com ","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	SignupHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp userCreatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != 42 {
		t.Fatalf("expected id 42, got %d", resp.ID)
	}
	if resp.Email != "test@email.com" {
		t.Fatalf("expected email normalized, got %q", resp.Email)
	}
	if resp.Token == "" {
		t.Fatal("expected token to be set")
	}
}

func TestSignupHandler_DuplicateEmail(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(_ ...any) error {
					return &pgconn.PgError{Code: "23505"}
				},
			}
		},
	}

	body := `{"email":"test@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	SignupHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}

	var resp apiError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "email already exists" {
		t.Fatalf("unexpected error %q", resp.Error)
	}
}

func TestLoginHandler_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	hash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 99
					*dest[1].(*string) = string(hash)
					*dest[2].(*bool) = true
					return nil
				},
			}
		},
	}

	body := `{"email":"user@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	LoginHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp authResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token to be set")
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{
				scanFn: func(_ ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	body := `{"email":"user@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	LoginHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp apiError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "invalid credentials" {
		t.Fatalf("unexpected error %q", resp.Error)
	}
}
