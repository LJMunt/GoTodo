package logging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// StartConfigWatcher periodically checks the database for configuration inconsistencies and logs warnings.
func StartConfigWatcher(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, every time.Duration) {
	check := func() {
		checkConfigSanity(ctx, logger, pool)
	}

	check() // initial check

	t := time.NewTicker(every)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				check()
			}
		}
	}()
}

func checkConfigSanity(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		SELECT key, value_json 
		FROM config_keys 
		WHERE key IN (
			'auth.requireEmailVerification',
			'auth.allowSignup',
			'mail.enabled',
			'mail.smtp.host',
			'mail.smtp.port'
		)
	`)
	if err != nil {
		logger.Debug().Err(err).Msg("config sanity check: failed to fetch config keys")
		return
	}
	defer rows.Close()

	values := make(map[string][]byte)
	for rows.Next() {
		var key string
		var val []byte
		if err := rows.Scan(&key, &val); err != nil {
			continue
		}
		values[key] = val
	}

	if err := rows.Err(); err != nil {
		return
	}

	getBool := func(key string) bool {
		v, ok := values[key]
		if !ok || v == nil {
			return false
		}
		var b bool
		_ = json.Unmarshal(v, &b)
		return b
	}

	getString := func(key string) string {
		v, ok := values[key]
		if !ok || v == nil {
			return ""
		}
		var s string
		_ = json.Unmarshal(v, &s)
		return s
	}

	getInt := func(key string) int {
		v, ok := values[key]
		if !ok || v == nil {
			return 0
		}
		var i int
		_ = json.Unmarshal(v, &i)
		return i
	}

	requireVerification := getBool("auth.requireEmailVerification")
	mailEnabled := getBool("mail.enabled")
	allowSignup := getBool("auth.allowSignup")

	if requireVerification && !mailEnabled {
		if allowSignup {
			logger.Warn().
				Str("event", "config.sanity_check.warning").
				Msg("CRITICAL CONFIGURATION ISSUE: New user signup is ENABLED and email verification is REQUIRED, but mail sending is DISABLED. New users will be unable to verify their accounts or log in.")
		} else {
			logger.Warn().
				Str("event", "config.sanity_check.warning").
				Msg("Configuration warning: Email verification is REQUIRED but mail sending is DISABLED. Existing unverified users will be unable to log in.")
		}
	}

	if mailEnabled {
		host := getString("mail.smtp.host")
		port := getInt("mail.smtp.port")
		if host == "" || port == 0 {
			logger.Warn().
				Str("event", "config.sanity_check.warning").
				Msg("Mail sending is ENABLED, but SMTP host or port is not configured. Outbound emails will likely fail.")
		}
	}
}
