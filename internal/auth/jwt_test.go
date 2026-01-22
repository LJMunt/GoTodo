package auth

import (
	"os"
	"testing"
	"time"
)

func TestJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

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

	t.Run("InvalidToken", func(t *testing.T) {
		_, err := ParseToken("invalid-token-string")
		if err == nil {
			t.Error("expected error for invalid token")
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
