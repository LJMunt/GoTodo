package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"time"

	authmw "GoToDo/internal/auth"
	"GoToDo/internal/logging"
	"GoToDo/internal/secrets"

	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type mfaTotpVerifyRequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
}

func MfaTotpVerifyHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req mfaTotpVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		l := logging.From(ctx)

		claims, err := authmw.ParseToken(req.MFAToken)
		if err != nil || claims.Type != "mfa" {
			l.Debug().Err(err).Msg("invalid mfa token")
			writeErr(w, http.StatusUnauthorized, "invalid mfa token")
			return
		}

		jtiHash := hashToken(claims.ID)

		var (
			challengeUserID  int64
			expiresAt        time.Time
			consumedAt       *time.Time
			failCount        int
			challengeIP      netip.Addr
			totpSecretEnc    *string
			totpLastUsedStep *int64
			tokenVersion     int64
		)

		err = db.QueryRow(ctx,
			`SELECT c.user_id, c.expires_at, c.consumed_at, c.fail_count, c.ip,
			        u.totp_secret_enc, u.totp_last_used_step, u.token_version
			 FROM mfa_challenges c
			 JOIN users u ON c.user_id = u.id
			 WHERE c.jti_hash = $1`,
			jtiHash,
		).Scan(&challengeUserID, &expiresAt, &consumedAt, &failCount, &challengeIP,
			&totpSecretEnc, &totpLastUsedStep, &tokenVersion)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				l.Debug().Str("jti_hash", jtiHash).Msg("mfa challenge not found")
				writeErr(w, http.StatusUnauthorized, "mfa challenge not found")
			} else {
				l.Error().Err(err).Str("jti_hash", jtiHash).Msg("database error fetching mfa challenge")
				writeErr(w, http.StatusInternalServerError, "database error")
			}
			return
		}

		if challengeUserID != claims.UserID {
			l.Warn().
				Int64("challenge_user_id", challengeUserID).
				Int64("token_user_id", claims.UserID).
				Msg("mfa token user mismatch")
			writeErr(w, http.StatusUnauthorized, "invalid mfa token")
			return
		}

		if consumedAt != nil {
			l.Warn().Str("jti_hash", jtiHash).Msg("mfa token already consumed")
			writeErr(w, http.StatusUnauthorized, "mfa token already consumed")
			return
		}

		if time.Now().After(expiresAt) {
			l.Warn().
				Str("jti_hash", jtiHash).
				Time("expires_at", expiresAt).
				Msg("mfa token expired")
			writeErr(w, http.StatusUnauthorized, "mfa token expired")
			return
		}

		if failCount >= 5 {
			l.Warn().
				Int64("user_id", challengeUserID).
				Int("fail_count", failCount).
				Msg("mfa verification blocked: too many failed attempts")
			writeErr(w, http.StatusUnauthorized, "too many failed attempts")
			return
		}

		// Check if TOTP is allowed system-wide
		allowTOTP, err := isTOTPAllowed(ctx, db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read mfa settings")
			return
		}
		if !allowTOTP {
			writeErr(w, http.StatusForbidden, "TOTP is currently disabled")
			return
		}

		// Verify IP matches
		requestIP := extractIP(r.RemoteAddr)
		if challengeIP != requestIP {
			l.Warn().
				Str("challenge_ip", challengeIP.String()).
				Str("request_ip", requestIP.String()).
				Msg("mfa verification IP mismatch")
			writeErr(w, http.StatusUnauthorized, "invalid mfa session")
			return
		}

		if totpSecretEnc == nil {
			writeErr(w, http.StatusUnauthorized, "totp not enabled")
			return
		}

		masterKey, err := secrets.LoadMasterKey()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to load master key")
			return
		}

		secret, err := secrets.DecryptString(*totpSecretEnc, masterKey, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to decrypt totp secret")
			return
		}

		// Verify TOTP with actual step detection for replay protection
		// We check T-1, T, T+1 to match totp.Validate's default skew of 1
		now := time.Now().UTC()
		currentStep := now.Unix() / 30
		var validatedStep int64
		var valid bool

		for i := int64(-1); i <= 1; i++ {
			step := currentStep + i
			v, err := totp.ValidateCustom(req.Code, secret, time.Unix(step*30, 0).UTC(), totp.ValidateOpts{
				Period:    30,
				Skew:      0,
				Digits:    6,
				Algorithm: otp.AlgorithmSHA1,
			})
			if err == nil && v {
				valid = true
				validatedStep = step
				break
			}
		}

		if !valid {
			l.Debug().Int64("user_id", challengeUserID).Msg("invalid totp code attempt")
			_, _ = db.Exec(ctx, "UPDATE mfa_challenges SET fail_count = fail_count + 1 WHERE jti_hash = $1", jtiHash)
			writeErr(w, http.StatusUnauthorized, "invalid totp code")
			return
		}

		// Check if this step was already used
		if totpLastUsedStep != nil && *totpLastUsedStep >= validatedStep {
			l.Warn().
				Int64("user_id", challengeUserID).
				Int64("last_step", *totpLastUsedStep).
				Int64("current_step", validatedStep).
				Msg("totp code replay detected")
			writeErr(w, http.StatusUnauthorized, "code already used")
			return
		}

		// Update challenge as consumed and user's last used step in a transaction
		beginner, ok := db.(interface {
			Begin(ctx context.Context) (pgx.Tx, error)
		})
		if !ok {
			// Fallback if not a pool that supports Begin
			_, _ = db.Exec(ctx, "UPDATE mfa_challenges SET consumed_at = now() WHERE jti_hash = $1", jtiHash)
			_, _ = db.Exec(ctx, "UPDATE users SET totp_last_used_step = $1 WHERE id = $2", validatedStep, claims.UserID)
		} else {
			tx, err := beginner.Begin(ctx)
			if err != nil {
				l.Error().Err(err).Msg("failed to start transaction for mfa verification")
				writeErr(w, http.StatusInternalServerError, "failed to start transaction")
				return
			}
			defer tx.Rollback(ctx)

			if _, err := tx.Exec(ctx, "UPDATE mfa_challenges SET consumed_at = now() WHERE jti_hash = $1", jtiHash); err != nil {
				l.Error().Err(err).Msg("failed to update mfa challenge in transaction")
				writeErr(w, http.StatusInternalServerError, "failed to update challenge")
				return
			}
			if _, err := tx.Exec(ctx, "UPDATE users SET totp_last_used_step = $1 WHERE id = $2", validatedStep, claims.UserID); err != nil {
				l.Error().Err(err).Msg("failed to update user totp_last_used_step in transaction")
				writeErr(w, http.StatusInternalServerError, "failed to update user")
				return
			}

			if err := tx.Commit(ctx); err != nil {
				l.Error().Err(err).Msg("failed to commit mfa verification transaction")
				writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
				return
			}
		}

		l.Info().Int64("user_id", claims.UserID).Msg("mfa totp verification successful")
		token, err := authmw.SignAccessToken(claims.UserID, tokenVersion, true)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token})
	}
}

type totpStartResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

func MfaTotpStartHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		l := logging.From(ctx)

		// Check if TOTP is allowed
		allowTOTP, err := isTOTPAllowed(ctx, db)
		if err != nil {
			l.Error().Err(err).Msg("failed to check if totp is allowed")
			writeErr(w, http.StatusInternalServerError, "failed to read mfa settings")
			return
		}
		if !allowTOTP {
			l.Warn().Int64("user_id", user.ID).Msg("totp enrollment attempt while disabled")
			writeErr(w, http.StatusForbidden, "TOTP is currently disabled")
			return
		}

		var email string
		err = db.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", user.ID).Scan(&email)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("database error fetching user email for totp start")
			writeErr(w, http.StatusInternalServerError, "database error")
			return
		}

		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "GoToDo",
			AccountName: email,
		})
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to generate totp key")
			writeErr(w, http.StatusInternalServerError, "failed to generate totp key")
			return
		}

		masterKey, err := secrets.LoadMasterKey()
		if err != nil {
			l.Error().Err(err).Msg("failed to load master key for totp start")
			writeErr(w, http.StatusInternalServerError, "failed to load master key")
			return
		}

		encryptedSecret, err := secrets.EncryptString(key.Secret(), masterKey, nil)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to encrypt totp secret")
			writeErr(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}

		_, err = db.Exec(ctx,
			"UPDATE users SET totp_secret_enc = $1, totp_enabled = false, totp_confirmed_at = NULL WHERE id = $2",
			encryptedSecret, user.ID,
		)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to store totp secret")
			writeErr(w, http.StatusInternalServerError, "failed to store secret")
			return
		}

		l.Info().Int64("user_id", user.ID).Msg("totp enrollment started")
		writeJSON(w, http.StatusOK, totpStartResponse{
			Secret: key.Secret(),
			URL:    key.URL(),
		})
	}
}

type totpConfirmRequest struct {
	Code string `json:"code"`
}

type totpConfirmResponse struct {
	Status      string   `json:"status"`
	BackupCodes []string `json:"backup_codes"`
}

func MfaTotpConfirmHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req totpConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		l := logging.From(ctx)

		// Check if TOTP is allowed
		allowTOTP, err := isTOTPAllowed(ctx, db)
		if err != nil {
			l.Error().Err(err).Msg("failed to check if totp is allowed for confirmation")
			writeErr(w, http.StatusInternalServerError, "failed to read mfa settings")
			return
		}
		if !allowTOTP {
			l.Warn().Int64("user_id", user.ID).Msg("totp confirmation attempt while disabled")
			writeErr(w, http.StatusForbidden, "TOTP is currently disabled")
			return
		}

		var totpSecretEnc *string
		err = db.QueryRow(ctx, "SELECT totp_secret_enc FROM users WHERE id = $1", user.ID).Scan(&totpSecretEnc)
		if err != nil || totpSecretEnc == nil {
			l.Warn().Int64("user_id", user.ID).Msg("totp confirmation attempt without start")
			writeErr(w, http.StatusBadRequest, "totp setup not started")
			return
		}

		masterKey, err := secrets.LoadMasterKey()
		if err != nil {
			l.Error().Err(err).Msg("failed to load master key for totp confirmation")
			writeErr(w, http.StatusInternalServerError, "failed to load master key")
			return
		}

		secret, err := secrets.DecryptString(*totpSecretEnc, masterKey, nil)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to decrypt totp secret for confirmation")
			writeErr(w, http.StatusInternalServerError, "failed to decrypt secret")
			return
		}

		// Verify TOTP with actual step detection for replay protection
		now := time.Now().UTC()
		currentStep := now.Unix() / 30
		var validatedStep int64
		var valid bool

		for i := int64(-1); i <= 1; i++ {
			step := currentStep + i
			v, err := totp.ValidateCustom(req.Code, secret, time.Unix(step*30, 0).UTC(), totp.ValidateOpts{
				Period:    30,
				Skew:      0,
				Digits:    6,
				Algorithm: otp.AlgorithmSHA1,
			})
			if err == nil && v {
				valid = true
				validatedStep = step
				break
			}
		}

		if !valid {
			l.Debug().Int64("user_id", user.ID).Msg("invalid totp code during confirmation")
			writeErr(w, http.StatusUnauthorized, "invalid code")
			return
		}

		backupCodes, err := generateBackupCodes(10)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to generate backup codes during confirmation")
			writeErr(w, http.StatusInternalServerError, "failed to generate backup codes")
			return
		}

		// Update challenge as consumed and user's last used step in a transaction
		beginner, ok := db.(interface {
			Begin(ctx context.Context) (pgx.Tx, error)
		})
		if !ok {
			// Fallback if not a pool that supports Begin
			_, _ = db.Exec(ctx, "DELETE FROM backup_codes WHERE user_id = $1", user.ID)
			_, _ = db.Exec(ctx, "UPDATE users SET totp_enabled = true, totp_confirmed_at = now(), totp_last_used_step = $1 WHERE id = $2", validatedStep, user.ID)
			for _, code := range backupCodes {
				_, _ = db.Exec(ctx, "INSERT INTO backup_codes (user_id, code_hash) VALUES ($1, $2)", user.ID, hashToken(code))
			}
		} else {
			tx, err := beginner.Begin(ctx)
			if err != nil {
				l.Error().Err(err).Msg("failed to start transaction for totp confirmation")
				writeErr(w, http.StatusInternalServerError, "failed to start transaction")
				return
			}
			defer tx.Rollback(ctx)

			if _, err := tx.Exec(ctx, "DELETE FROM backup_codes WHERE user_id = $1", user.ID); err != nil {
				l.Error().Err(err).Msg("failed to clear old backup codes in transaction")
				writeErr(w, http.StatusInternalServerError, "failed to clear old backup codes")
				return
			}
			if _, err := tx.Exec(ctx, "UPDATE users SET totp_enabled = true, totp_confirmed_at = now(), totp_last_used_step = $1 WHERE id = $2", validatedStep, user.ID); err != nil {
				l.Error().Err(err).Msg("failed to update user during totp confirmation in transaction")
				writeErr(w, http.StatusInternalServerError, "failed to update user")
				return
			}
			for _, code := range backupCodes {
				if _, err := tx.Exec(ctx, "INSERT INTO backup_codes (user_id, code_hash) VALUES ($1, $2)", user.ID, hashToken(code)); err != nil {
					l.Error().Err(err).Msg("failed to store backup code in transaction")
					writeErr(w, http.StatusInternalServerError, "failed to store backup codes")
					return
				}
			}

			if err := tx.Commit(ctx); err != nil {
				l.Error().Err(err).Msg("failed to commit totp confirmation transaction")
				writeErr(w, http.StatusInternalServerError, "failed to commit transaction")
				return
			}
		}

		l.Info().Int64("user_id", user.ID).Msg("totp confirmed and enabled")
		writeJSON(w, http.StatusOK, totpConfirmResponse{Status: "ok", BackupCodes: backupCodes})
	}
}

func generateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := range count {
		b := make([]byte, 6) // 12 characters
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		codes[i] = hex.EncodeToString(b)
	}
	return codes, nil
}

func MfaTotpDisableHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		l := logging.From(ctx)

		_, err := db.Exec(ctx,
			"UPDATE users SET totp_enabled = false, totp_secret_enc = NULL, totp_confirmed_at = NULL WHERE id = $1",
			user.ID,
		)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to disable totp")
			writeErr(w, http.StatusInternalServerError, "failed to disable totp")
			return
		}

		_, err = db.Exec(ctx, "DELETE FROM backup_codes WHERE user_id = $1", user.ID)
		if err != nil {
			l.Error().Err(err).Int64("user_id", user.ID).Msg("failed to delete backup codes while disabling totp")
		}

		l.Info().Int64("user_id", user.ID).Msg("totp disabled")
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
