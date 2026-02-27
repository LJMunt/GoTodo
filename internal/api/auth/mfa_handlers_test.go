package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authmw "GoToDo/internal/auth"
	"GoToDo/internal/secrets"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pquerna/otp/totp"
)

func TestMfaTotpStartHandler(t *testing.T) {
	masterKey := make([]byte, 32)
	keyB64 := base64.StdEncoding.EncodeToString(masterKey)
	t.Setenv("SECRETS_MASTER_KEY_B64", keyB64)

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "config_keys") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*[]byte) = []byte("true")
					return nil
				}}
			}
			if strings.Contains(sql, "SELECT email") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "user@example.com"
					return nil
				}}
			}
			return fakeRow{}
		},
		execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "UPDATE users") {
				t.Errorf("unexpected SQL: %s", sql)
			}
			return pgconn.CommandTag{}, nil
		},
	}

	handler := MfaTotpStartHandler(db)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mfa/totp/start", nil)

	// Add user to context
	user := authmw.User{ID: 1}
	req = req.WithContext(authmw.WithUser(req.Context(), user))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp totpStartResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Secret == "" || resp.URL == "" {
		t.Fatal("expected secret and url to be set")
	}
}

func TestMfaTotpConfirmHandler(t *testing.T) {
	masterKey := make([]byte, 32)
	keyB64 := base64.StdEncoding.EncodeToString(masterKey)
	t.Setenv("SECRETS_MASTER_KEY_B64", keyB64)

	secret := "JBSWY3DPEHPK3PXP"
	encryptedSecret, _ := secrets.EncryptString(secret, masterKey, nil)
	code, _ := totp.GenerateCode(secret, time.Now())

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "config_keys") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*[]byte) = []byte("true")
					return nil
				}}
			}
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(**string) = &encryptedSecret
				return nil
			}}
		},
		execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "UPDATE users") {
				t.Errorf("unexpected SQL: %s", sql)
			}
			return pgconn.CommandTag{}, nil
		},
	}

	handler := MfaTotpConfirmHandler(db)
	body := map[string]string{"code": code}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mfa/totp/confirm", bytes.NewBuffer(b))
	req = req.WithContext(authmw.WithUser(req.Context(), authmw.User{ID: 1}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestMfaTotpVerifyHandler_Success(t *testing.T) {
	masterKey := make([]byte, 32)
	keyB64 := base64.StdEncoding.EncodeToString(masterKey)
	t.Setenv("SECRETS_MASTER_KEY_B64", keyB64)
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_ISSUER", "gotodo-test")
	t.Setenv("JWT_AUDIENCE", "gotodo-test-client")

	secret := "JBSWY3DPEHPK3PXP"
	encryptedSecret, _ := secrets.EncryptString(secret, masterKey, nil)
	code, _ := totp.GenerateCode(secret, time.Now().UTC())

	mfaToken, jti, _ := authmw.SignMFAToken(1, 0)

	db := fakeAuthDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "config_keys") {
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*[]byte) = []byte("true")
					return nil
				}}
			}
			if strings.Contains(sql, "JOIN users") {
				expiresAt := time.Now().Add(5 * time.Minute).UTC()
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 1 // challengeUserID
					*dest[1].(*time.Time) = expiresAt
					*dest[2].(**time.Time) = nil     // consumedAt
					*dest[3].(*int) = 0              // failCount
					*dest[4].(*string) = "192.0.2.1" // challengeIP
					*dest[5].(**string) = &encryptedSecret
					*dest[6].(**int64) = nil // totpLastUsedStep
					*dest[7].(*int64) = 0    // tokenVersion
					return nil
				}}
			}
			return fakeRow{}
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	handler := MfaTotpVerifyHandler(db)
	body := mfaTotpVerifyRequest{
		MFAToken: mfaToken,
		Code:     code,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/totp", bytes.NewBuffer(b))
	req.RemoteAddr = "192.0.2.1:1234"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s, JTI: %s", http.StatusOK, rec.Code, rec.Body.String(), jti)
	}

	var resp authResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected access token to be set")
	}
}
