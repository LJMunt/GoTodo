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
	UserID       int64    `json:"user_id"`
	TokenVersion int64    `json:"token_version"`
	Type         string   `json:"typ,omitzero"`
	MFARequired  bool     `json:"mfa_required,omitzero"`
	MFAMethods   []string `json:"mfa_methods,omitzero"`
	AMR          []string `json:"amr,omitzero"`
	MFA          bool     `json:"mfa,omitzero"`
	MFAAt        int64    `json:"mfa_at,omitzero"`
	jwt.RegisteredClaims
}

func SignToken(userID int64, tokenVersion int64) (string, error) {
	return SignAccessToken(userID, tokenVersion, false)
}

func SignAccessToken(userID int64, tokenVersion int64, mfa bool) (string, error) {
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

	amr := []string{"pwd"}
	if mfa {
		amr = append(amr, "mfa")
	}

	var mfaAt int64
	if mfa {
		mfaAt = time.Now().Unix()
	}

	claims := Claims{
		UserID:       userID,
		TokenVersion: tokenVersion,
		Type:         "access",
		AMR:          amr,
		MFA:          mfa,
		MFAAt:        mfaAt,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ID:        jti,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func SignMFAToken(userID int64, tokenVersion int64) (string, string, error) {
	secret, issuer, audience, err := jwtConfig()
	if err != nil {
		return "", "", err
	}

	ttl := 5 * time.Minute

	jti, err := newJTI()
	if err != nil {
		return "", "", err
	}

	claims := Claims{
		UserID:       userID,
		TokenVersion: tokenVersion,
		Type:         "mfa",
		MFARequired:  true,
		MFAMethods:   []string{"totp"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ID:        jti,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, jti, err
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
