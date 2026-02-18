package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       int64 `json:"user_id"`
	TokenVersion int64 `json:"token_version"`
	jwt.RegisteredClaims
}

func SignToken(userID int64, tokenVersion int64) (string, error) {
	secret, issuer, audience, err := jwtConfig()
	if err != nil {
		return "", err
	}

	ttlStr := os.Getenv("JWT_ACCESS_TTL")
	ttl := 12 * time.Hour
	if ttlStr != "" {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			ttl = parsed
		}
	}

	jti, err := newJTI()
	if err != nil {
		return "", err
	}

	claims := Claims{
		UserID:       userID,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenString string) (*Claims, error) {
	secret, issuer, audience, err := jwtConfig()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithIssuer(issuer), jwt.WithAudience(audience))
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	if strings.TrimSpace(claims.ID) == "" {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func jwtConfig() (string, string, string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "", "", "", fmt.Errorf("JWT_SECRET is not set")
	}

	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		return "", "", "", fmt.Errorf("JWT_ISSUER is not set")
	}

	audience := strings.TrimSpace(os.Getenv("JWT_AUDIENCE"))
	if audience == "" {
		return "", "", "", fmt.Errorf("JWT_AUDIENCE is not set")
	}

	return secret, issuer, audience, nil
}

func newJTI() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
