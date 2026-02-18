package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type revokeDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func RevokeToken(ctx context.Context, db revokeDB, jti string, userID int64, expiresAt time.Time, reason string) error {
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return fmt.Errorf("missing jti")
	}
	if expiresAt.IsZero() {
		return fmt.Errorf("missing expires_at")
	}

	_, err := db.Exec(ctx, `
		INSERT INTO jwt_revocations (jti, user_id, expires_at, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (jti) DO NOTHING
	`, jti, userID, expiresAt, strings.TrimSpace(reason))
	return err
}
