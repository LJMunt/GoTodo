package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("JWT_ISSUER", "gotodo-test")
	os.Setenv("JWT_AUDIENCE", "gotodo-test-client")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("JWT_ISSUER")
	defer os.Unsetenv("JWT_AUDIENCE")

	t.Run("SignAndParseToken", func(t *testing.T) {
		userID := int64(123)
		token, err := SignToken(userID)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		claims, err := ParseToken(token)
		if err != nil {
			t.Fatalf("failed to parse token: %v", err)
		}

		if claims.UserID != userID {
			t.Errorf("expected userID %d, got %d", userID, claims.UserID)
		}
	})

	t.Run("MissingSecret", func(t *testing.T) {
		os.Unsetenv("JWT_SECRET")
		defer os.Setenv("JWT_SECRET", "test-secret")

		_, err := SignToken(123)
		if err == nil {
			t.Error("expected error when JWT_SECRET is missing")
		}

		_, err = ParseToken("some-token")
		if err == nil {
			t.Error("expected error when JWT_SECRET is missing")
		}
	})

	t.Run("MissingIssuerOrAudience", func(t *testing.T) {
		os.Unsetenv("JWT_ISSUER")
		defer os.Setenv("JWT_ISSUER", "gotodo-test")
		_, err := SignToken(123)
		if err == nil {
			t.Error("expected error when JWT_ISSUER is missing")
		}

		os.Setenv("JWT_ISSUER", "gotodo-test")
		os.Unsetenv("JWT_AUDIENCE")
		defer os.Setenv("JWT_AUDIENCE", "gotodo-test-client")
		_, err = SignToken(123)
		if err == nil {
			t.Error("expected error when JWT_AUDIENCE is missing")
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		_, err := ParseToken("invalid-token-string")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("RejectsUnexpectedAlgorithm", func(t *testing.T) {
		claims := Claims{
			UserID: 123,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
		tokenString, err := token.SignedString([]byte("test-secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = ParseToken(tokenString)
		if err == nil {
			t.Error("expected error for unexpected signing algorithm")
		}
	})

	t.Run("TokenTTL", func(t *testing.T) {
		os.Setenv("JWT_ACCESS_TTL", "1s")
		defer os.Unsetenv("JWT_ACCESS_TTL")

		token, err := SignToken(123)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		time.Sleep(1100 * time.Millisecond)

		_, err = ParseToken(token)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})
}
