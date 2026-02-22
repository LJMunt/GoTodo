package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"GoToDo/internal/mail"
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
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (db fakeAuthDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if db.queryFn == nil {
		return nil, nil
	}
	return db.queryFn(ctx, sql, args...)
}

func (db fakeAuthDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if db.queryRowFn == nil {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	return db.queryRowFn(ctx, sql, args...)
}

func (db fakeAuthDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if db.execFn == nil {
		return pgconn.CommandTag{}, nil
	}
	return db.execFn(ctx, sql, args...)
}

func TestSignupHandler_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_ISSUER", "gotodo-test")
	t.Setenv("JWT_AUDIENCE", "gotodo-test-client")

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.allowSignup'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*string) = "true"
						return nil
					},
				}
			}
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*[]byte) = []byte("false")
						return nil
					},
				}
			}
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 42
					*dest[1].(*int64) = 0
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
	if resp.PublicID == "" {
		t.Fatal("expected public_id to be set")
	}
	if len(resp.PublicID) != 26 {
		t.Fatalf("expected public_id length 26, got %d", len(resp.PublicID))
	}
	if resp.Email != "test@email.com" {
		t.Fatalf("expected email normalized, got %q", resp.Email)
	}
	if resp.VerificationRequired {
		t.Fatal("expected verificationRequired to be false")
	}
	if resp.Token == "" {
		t.Fatal("expected token to be set")
	}
}

func TestSignupHandler_DuplicateEmail(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.allowSignup'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*string) = "true"
						return nil
					},
				}
			}
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*[]byte) = []byte("false")
						return nil
					},
				}
			}
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

func TestSignupHandler_Disabled(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.allowSignup'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*string) = "false"
						return nil
					},
				}
			}
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*[]byte) = []byte("false")
						return nil
					},
				}
			}
			return fakeRow{
				scanFn: func(_ ...any) error {
					return nil
				},
			}
		},
	}

	body := `{"email":"test@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	SignupHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	var resp apiError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "signup_disabled" {
		t.Fatalf("expected error signup_disabled, got %q", resp.Error)
	}
	if resp.Message != "New account registration is currently disabled." {
		t.Fatalf("unexpected message: %q", resp.Message)
	}
	if resp.Retryable != false {
		t.Fatal("expected retryable to be false")
	}
}

func TestLoginHandler_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_ISSUER", "gotodo-test")
	t.Setenv("JWT_AUDIENCE", "gotodo-test-client")

	hash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*[]byte) = []byte("false")
						return nil
					},
				}
			}
			return fakeRow{
				scanFn: func(dest ...any) error {
					now := time.Now()
					*dest[0].(*int64) = 99
					*dest[1].(*string) = string(hash)
					*dest[2].(*bool) = true
					*dest[3].(*bool) = false
					*dest[4].(**time.Time) = &now
					*dest[5].(*int64) = 0
					return nil
				},
			}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
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

func TestLoginHandler_UnverifiedEmailBlocked(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_ISSUER", "gotodo-test")
	t.Setenv("JWT_AUDIENCE", "gotodo-test-client")

	hash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if sql == "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'" {
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*[]byte) = []byte("true")
						return nil
					},
				}
			}
			return fakeRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 99
					*dest[1].(*string) = string(hash)
					*dest[2].(*bool) = true
					*dest[3].(*bool) = false
					*dest[4].(**time.Time) = nil
					*dest[5].(*int64) = 0
					return nil
				},
			}
		},
	}

	body := `{"email":"user@example.com","password":"Password123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	LoginHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
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

func TestVerifyEmailHandler_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_ISSUER", "gotodo-test")
	t.Setenv("JWT_AUDIENCE", "gotodo-test-client")
	token := "abc123"
	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(1 * time.Hour)

	var execCalls int
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "FROM email_verification_tokens") {
				if len(args) != 1 || args[0] != tokenHash {
					return fakeRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }}
				}
				return fakeRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*int64) = 1
						*dest[1].(*int64) = 7
						*dest[2].(*time.Time) = expiresAt
						*dest[3].(**time.Time) = nil
						*dest[4].(*string) = "user@example.com"
						*dest[5].(*string) = "user@example.com"
						*dest[6].(**time.Time) = nil
						return nil
					},
				}
			}
			return fakeRow{scanFn: func(_ ...any) error { return nil }}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCalls++
			return pgconn.CommandTag{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify-email?token="+token, nil)
	rec := httptest.NewRecorder()

	VerifyEmailHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if execCalls != 2 {
		t.Fatalf("expected 2 exec calls, got %d", execCalls)
	}
}

func TestVerifyEmailHandler_InvalidToken(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify-email?token=bad", nil)
	rec := httptest.NewRecorder()

	VerifyEmailHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestResendVerificationHandler_NoEmail(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scanFn: func(_ ...any) error { return nil }}
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email/resend", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()

	ResendVerificationHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestResendVerificationHandler_SendsWhenRequired(t *testing.T) {
	origSend := sendMail
	t.Cleanup(func() { sendMail = origSend })
	var sent mail.Message
	sendMail = func(_ context.Context, _ mailDB, msg mail.Message) error {
		sent = msg
		return nil
	}

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			switch {
			case sql == "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'":
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*[]byte) = []byte("true")
					return nil
				}}
			case strings.Contains(sql, "FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 9
					*dest[1].(**time.Time) = nil
					return nil
				}}
			case sql == "SELECT value_json FROM config_keys WHERE key = $1":
				key := args[0].(string)
				var val string
				switch key {
				case "instance.url":
					val = "http://localhost:8080"
				case "mail.verificationsubject":
					val = "Verify"
				case "mail.verificationbody":
					val = "Click {{.VerifyURL}}"
				}
				raw, _ := json.Marshal(val)
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*[]byte) = raw
					return nil
				}}
			}
			return fakeRow{scanFn: func(_ ...any) error { return nil }}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	body := `{"email":"user@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email/resend", bytes.NewBufferString(body))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	ResendVerificationHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if len(sent.To) != 1 || sent.To[0] != "user@example.com" {
		t.Fatalf("expected email to be sent")
	}
}

func TestPasswordReset_FullFlow(t *testing.T) {
	var storedSelector string
	var storedTokenHash string
	var updatedPasswordHash string

	db := fakeAuthDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			// Rate limit check
			if strings.Contains(sql, "FROM password_reset_tokens") && strings.Contains(sql, "count(*)") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				}}
			}
			// User lookup
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "id") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 123
					return nil
				}}
			}
			// Config lookups
			if strings.Contains(sql, "FROM config_keys") {
				return fakeRow{scanFn: func(dest ...any) error {
					if len(args) > 0 && args[0] == "auth.allowReset" {
						*dest[0].(*[]byte) = []byte("true")
						return nil
					}
					*dest[0].(*[]byte) = []byte(`""`)
					return nil
				}}
			}
			// Validate/Confirm lookups
			if strings.Contains(sql, "FROM password_reset_tokens") {
				if strings.Contains(sql, "token_hash, expires_at") {
					return fakeRow{scanFn: func(dest ...any) error {
						if args[0].(string) == storedSelector {
							*dest[0].(*string) = storedTokenHash
							*dest[1].(*time.Time) = time.Now().Add(time.Hour)
							return nil
						}
						return pgx.ErrNoRows
					}}
				}
				if strings.Contains(sql, "user_id, token_hash") {
					return fakeRow{scanFn: func(dest ...any) error {
						if args[0].(string) == storedSelector {
							*dest[0].(*int64) = 123
							*dest[1].(*string) = storedTokenHash
							return nil
						}
						return pgx.ErrNoRows
					}}
				}
			}
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			// Token storage
			if strings.Contains(sql, "INSERT INTO password_reset_tokens") {
				storedSelector = args[1].(string)
				storedTokenHash = args[2].(string)
			}
			// Password update
			if strings.Contains(sql, "UPDATE users") {
				updatedPasswordHash = args[0].(string)
			}
			return pgconn.CommandTag{}, nil
		},
	}

	// Mock token generation
	oldNewToken := newPasswordResetToken
	defer func() { newPasswordResetToken = oldNewToken }()
	newPasswordResetToken = func() (string, string, string, error) {
		return "sel123", "tok123", hashToken("tok123"), nil
	}

	// Mock mail sending
	oldSendMail := sendMail
	defer func() { sendMail = oldSendMail }()
	sendMail = func(ctx context.Context, db mailDB, msg mail.Message) error {
		return nil
	}

	// 1. Request reset
	reqBody, _ := json.Marshal(map[string]string{"email": "test@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewBuffer(reqBody))
	rec := httptest.NewRecorder()
	RequestPasswordResetHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if storedSelector != "sel123" {
		t.Fatalf("expected selector 'sel123', got %q", storedSelector)
	}

	// 2. Validate token
	valBody, _ := json.Marshal(map[string]string{"selector": "sel123", "token": "tok123"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/validate", bytes.NewBuffer(valBody))
	rec = httptest.NewRecorder()
	ValidatePasswordResetHandler(db).ServeHTTP(rec, req)

	var valResp struct {
		Valid bool `json:"valid"`
	}
	json.Unmarshal(rec.Body.Bytes(), &valResp)
	if !valResp.Valid {
		t.Error("expected token to be valid")
	}

	// 3. Confirm
	confBody, _ := json.Marshal(map[string]string{
		"selector":    "sel123",
		"token":       "tok123",
		"newPassword": "NewPassword123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewBuffer(confBody))
	rec = httptest.NewRecorder()
	ConfirmPasswordResetHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var confResp struct {
		OK bool `json:"ok"`
	}
	json.Unmarshal(rec.Body.Bytes(), &confResp)
	if !confResp.OK {
		t.Error("expected ok: true")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(updatedPasswordHash), []byte("NewPassword123!")); err != nil {
		t.Errorf("password hash mismatch: %v", err)
	}
}

func TestPasswordReset_InvalidToken(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	confBody, _ := json.Marshal(map[string]string{
		"selector":    "invalid",
		"token":       "invalid",
		"newPassword": "NewPassword123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewBuffer(confBody))
	rec := httptest.NewRecorder()
	ConfirmPasswordResetHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	var confResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &confResp)
	if confResp.OK {
		t.Error("expected ok: false")
	}
	if confResp.Error != "invalid_or_expired" {
		t.Errorf("expected error 'invalid_or_expired', got %q", confResp.Error)
	}
}

func TestPasswordReset_WeakPassword(t *testing.T) {
	db := fakeAuthDB{}
	confBody, _ := json.Marshal(map[string]string{
		"selector":    "sel123",
		"token":       "tok123",
		"newPassword": "weak",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewBuffer(confBody))
	rec := httptest.NewRecorder()
	ConfirmPasswordResetHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	var confResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &confResp)
	if confResp.Error != "password_too_weak" {
		t.Errorf("expected error 'password_too_weak', got %q", confResp.Error)
	}
}

func TestPasswordReset_Disabled(t *testing.T) {
	db := fakeAuthDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "auth.allowReset") || (len(args) > 0 && args[0] == "auth.allowReset") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*[]byte) = []byte("false")
					return nil
				}}
			}
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	// 1. Request
	reqBody, _ := json.Marshal(map[string]string{"email": "test@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewBuffer(reqBody))
	rec := httptest.NewRecorder()
	RequestPasswordResetHandler(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("Request: expected 403, got %d", rec.Code)
	}

	// 2. Validate
	valBody, _ := json.Marshal(map[string]string{"selector": "sel", "token": "tok"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/validate", bytes.NewBuffer(valBody))
	rec = httptest.NewRecorder()
	ValidatePasswordResetHandler(db).ServeHTTP(rec, req)
	var valResp struct {
		Valid bool `json:"valid"`
	}
	json.Unmarshal(rec.Body.Bytes(), &valResp)
	if valResp.Valid {
		t.Error("Validate: expected valid=false")
	}

	// 3. Confirm
	confBody, _ := json.Marshal(map[string]string{"selector": "sel", "token": "tok", "newPassword": "NewPassword123!"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewBuffer(confBody))
	rec = httptest.NewRecorder()
	ConfirmPasswordResetHandler(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("Confirm: expected 403, got %d", rec.Code)
	}
}

func TestPasswordReset_TemplateFields(t *testing.T) {
	var sent mail.Message
	db := fakeAuthDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "FROM password_reset_tokens") && strings.Contains(sql, "count(*)") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				}}
			}
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "id") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 123
					return nil
				}}
			}
			if strings.Contains(sql, "FROM config_keys") {
				return fakeRow{scanFn: func(dest ...any) error {
					key := args[0].(string)
					switch key {
					case "auth.allowReset":
						*dest[0].(*[]byte) = []byte("true")
					case "instance.url":
						*dest[0].(*[]byte) = []byte(`"https://example.com"`)
					case "mail.reset_password_subject":
						*dest[0].(*[]byte) = []byte(`"Reset for {{.Email}}"`)
					case "mail.reset_password_body":
						*dest[0].(*[]byte) = []byte(`"Go to {{.InstanceURL}} and use {{.ResetURL}}"`)
					default:
						*dest[0].(*[]byte) = []byte(`""`)
					}
					return nil
				}}
			}
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	// Mock mail sending
	oldSendMail := sendMail
	defer func() { sendMail = oldSendMail }()
	sendMail = func(ctx context.Context, db mailDB, msg mail.Message) error {
		sent = msg
		return nil
	}

	reqBody, _ := json.Marshal(map[string]string{"email": "user@test.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewBuffer(reqBody))
	rec := httptest.NewRecorder()
	RequestPasswordResetHandler(db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if sent.Subject != "Reset for user@test.com" {
		t.Errorf("expected subject 'Reset for user@test.com', got %q", sent.Subject)
	}
	if !strings.Contains(sent.HTML, "Go to https://example.com") {
		t.Errorf("expected HTML to contain InstanceURL, got %q", sent.HTML)
	}
	if !strings.Contains(sent.HTML, "use https://example.com/reset-password?selector=") {
		t.Errorf("expected HTML to contain ResetURL, got %q", sent.HTML)
	}
}
