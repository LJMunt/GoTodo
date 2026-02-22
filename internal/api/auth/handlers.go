package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"GoToDo/internal/app"
	authmw "GoToDo/internal/auth"
	"GoToDo/internal/logging"
	"GoToDo/internal/mail"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

type userCreatedResponse struct {
	PublicID             string `json:"public_id"`
	Email                string `json:"email"`
	Token                string `json:"token,omitempty"`
	VerificationRequired bool   `json:"verificationRequired"`
}

type apiError struct {
	Error     string `json:"error"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

type authDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

const minPasswordLen = 8
const verificationTokenTTL = 24 * time.Hour

type mailDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

var sendMail = func(ctx context.Context, db mailDB, msg mail.Message) error {
	return mail.Send(ctx, db, msg)
}

type verificationTemplateData struct {
	VerifyURL   string
	Email       string
	InstanceURL string
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case 'a' <= char && char <= 'z':
			hasLower = true
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()-_=+[]{}|;:,.<>/?", char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return errors.New("password must contain uppercase, lowercase, number and special character")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func SignupHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check if signup is allowed
		var val string
		err := db.QueryRow(ctx, "SELECT value_json FROM config_keys WHERE key = 'auth.allowSignup'").Scan(&val)
		if err == nil {
			if strings.Trim(val, "\"") == "false" {
				writeJSON(w, http.StatusForbidden, apiError{
					Error:     "signup_disabled",
					Message:   "New account registration is currently disabled.",
					Retryable: false,
				})
				return
			}
		}

		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" {
			writeErr(w, http.StatusBadRequest, "email is required")
			return
		}

		l := logging.From(ctx)
		l.Info().Str("email", email).Msg("user signup attempt")

		if err := validatePassword(req.Password); err != nil {
			l.Debug().Err(err).Str("email", email).Msg("signup failed: invalid password")
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		requireVerification, err := requireEmailVerification(ctx, db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read email verification setting")
			return
		}

		publicID := app.NewULID()

		ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var id int64
		var tokenVersion int64
		if requireVerification {
			err = db.QueryRow(ctx,
				`INSERT INTO users (email, password_hash, public_id) VALUES ($1, $2, $3)
				 RETURNING id, token_version`,
				email, string(hashedPassword), publicID,
			).Scan(&id, &tokenVersion)
		} else {
			err = db.QueryRow(ctx,
				`INSERT INTO users (email, password_hash, email_verified_at, public_id) VALUES ($1, $2, NOW(), $3)
				 RETURNING id, token_version`,
				email, string(hashedPassword), publicID,
			).Scan(&id, &tokenVersion)
		}

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
				l.Debug().Str("email", email).Msg("signup failed: email already exists")
				writeErr(w, http.StatusConflict, "email already exists")
				return
			}
			l.Error().Err(err).Str("email", email).Msg("signup failed: database error")
			writeErr(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		l.Info().Int64("user_id", id).Str("email", email).Msg("user created successfully")

		resp := userCreatedResponse{
			PublicID:             publicID,
			Email:                email,
			VerificationRequired: requireVerification,
		}

		if requireVerification {
			_ = createAndSendVerification(ctx, db, r, id, email)
			writeJSON(w, http.StatusCreated, resp)
			return
		}

		token, err := authmw.SignToken(id, tokenVersion)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}
		resp.Token = token

		writeJSON(w, http.StatusCreated, resp)
	}
}

func LoginHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" || req.Password == "" {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		l := logging.From(ctx)
		l.Info().Str("email", email).Msg("user login attempt")

		var (
			id              int64
			passwordHash    string
			isActive        bool
			isAdmin         bool
			emailVerifiedAt *time.Time
			tokenVersion    int64
		)

		err := db.QueryRow(ctx,
			`SELECT id, password_hash, is_active, is_admin, email_verified_at, token_version
			 FROM users
			 WHERE email=$1`,
			email,
		).Scan(&id, &passwordHash, &isActive, &isAdmin, &emailVerifiedAt, &tokenVersion)

		// Don’t leak whether email exists.
		if err != nil || !isActive {
			l.Debug().Str("email", email).Err(err).Msg("login failed: user not found or inactive")
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			l.Debug().Str("email", email).Msg("login failed: incorrect password")
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		requireVerification, err := requireEmailVerification(ctx, db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read email verification setting")
			return
		}
		if requireVerification && !isAdmin && emailVerifiedAt == nil {
			l.Debug().Str("email", email).Msg("login failed: email not verified")
			writeJSON(w, http.StatusForbidden, apiError{
				Error:     "email_not_verified",
				Message:   "Please verify your email before logging in.",
				Retryable: false,
			})
			return
		}

		// Update last_login
		_, _ = db.Exec(ctx, "UPDATE users SET last_login = now() WHERE id = $1", id)

		l.Info().Int64("user_id", id).Str("email", email).Msg("user login successful")

		token, err := authmw.SignToken(id, tokenVersion)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: token})
	}
}

func VerifyEmailHandler(db authDB) http.HandlerFunc {
	type verifyRequest struct {
		Token string `json:"token"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" && r.Body != nil {
			var req verifyRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				token = strings.TrimSpace(req.Token)
			}
		}
		if token == "" {
			writeErr(w, http.StatusBadRequest, "token is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		l := logging.From(ctx)
		tokenHash := hashToken(token)
		var (
			tokenID         int64
			userID          int64
			expiresAt       time.Time
			usedAt          *time.Time
			sentToEmail     string
			currentEmail    string
			emailVerifiedAt *time.Time
		)

		err := db.QueryRow(ctx, `
			SELECT t.id, t.user_id, t.expires_at, t.used_at, t.sent_to_email, u.email, u.email_verified_at
			FROM email_verification_tokens t
			JOIN users u ON u.id = t.user_id
			WHERE t.token_hash = $1
		`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &usedAt, &sentToEmail, &currentEmail, &emailVerifiedAt)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid or expired token")
			return
		}

		if usedAt != nil {
			writeErr(w, http.StatusBadRequest, "token already used")
			return
		}
		if time.Now().After(expiresAt) {
			writeErr(w, http.StatusBadRequest, "token expired")
			return
		}
		if !strings.EqualFold(sentToEmail, currentEmail) {
			writeErr(w, http.StatusBadRequest, "email mismatch")
			return
		}

		if emailVerifiedAt == nil {
			if _, err := db.Exec(ctx, "UPDATE users SET email_verified_at = NOW() WHERE id = $1", userID); err != nil {
				l.Error().Err(err).Int64("user_id", userID).Msg("failed to verify email")
				writeErr(w, http.StatusInternalServerError, "failed to verify email")
				return
			}
			l.Info().Int64("user_id", userID).Msg("email verified")
		}
		if _, err := db.Exec(ctx, "UPDATE email_verification_tokens SET used_at = NOW() WHERE id = $1", tokenID); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to finalize verification")
			return
		}

		var tokenVersion int64
		if err := db.QueryRow(ctx, "SELECT token_version FROM users WHERE id = $1", userID).Scan(&tokenVersion); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		signed, err := authmw.SignToken(userID, tokenVersion)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to sign token")
			return
		}

		writeJSON(w, http.StatusOK, authResponse{Token: signed})
	}
}

func ResendVerificationHandler(db authDB) http.HandlerFunc {
	type resendRequest struct {
		Email string `json:"email"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req resendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" {
			writeErr(w, http.StatusBadRequest, "email is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		requireVerification, err := requireEmailVerification(ctx, db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to read email verification setting")
			return
		}
		if !requireVerification {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var (
			userID          int64
			emailVerifiedAt *time.Time
		)
		err = db.QueryRow(ctx, `
			SELECT id, email_verified_at
			FROM users
			WHERE email = $1 AND is_active = true
		`, email).Scan(&userID, &emailVerifiedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			writeErr(w, http.StatusInternalServerError, "failed to look up user")
			return
		}
		if emailVerifiedAt != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := createAndSendVerification(ctx, db, r, userID, email); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to send verification email")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func requireEmailVerification(ctx context.Context, db authDB) (bool, error) {
	var raw []byte
	if err := db.QueryRow(ctx, "SELECT value_json FROM config_keys WHERE key = 'auth.requireEmailVerification'").Scan(&raw); err != nil {
		return false, err
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, err
	}
	return v, nil
}

func isPasswordResetAllowed(ctx context.Context, db authDB) (bool, error) {
	var raw []byte
	err := db.QueryRow(ctx, "SELECT value_json FROM config_keys WHERE key = $1", "auth.allowReset").Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, err
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 == nil {
			return s == "true", nil
		}
		return false, err
	}
	return v, nil
}

func getConfigString(ctx context.Context, db authDB, key string) (string, error) {
	var raw []byte
	if err := db.QueryRow(ctx, "SELECT value_json FROM config_keys WHERE key = $1", key).Scan(&raw); err != nil {
		return "", err
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	return v, nil
}

func createAndSendVerification(ctx context.Context, db authDB, r *http.Request, userID int64, email string) error {
	token, tokenHash, err := newVerificationToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(verificationTokenTTL)
	ip := extractIP(r.RemoteAddr)
	userAgent := strings.TrimSpace(r.UserAgent())

	if _, err := db.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at, sent_to_email, ip_created, user_agent_created)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, tokenHash, expiresAt, email, ip, userAgent); err != nil {
		return err
	}

	instanceURL, err := getConfigString(ctx, db, "instance.url")
	if err != nil {
		return err
	}
	subject, err := getConfigString(ctx, db, "mail.verificationsubject")
	if err != nil {
		return err
	}
	body, err := getConfigString(ctx, db, "mail.verificationbody")
	if err != nil {
		return err
	}

	verifyURL := strings.TrimRight(instanceURL, "/") + "/verify-email?token=" + url.QueryEscape(token)
	data := verificationTemplateData{
		VerifyURL:   verifyURL,
		Email:       email,
		InstanceURL: instanceURL,
	}

	renderedSubject, err := renderTemplate("verification_subject", subject, data)
	if err != nil {
		return err
	}
	renderedBody, err := renderTemplate("verification_body", body, data)
	if err != nil {
		return err
	}

	msg := mail.Message{
		To:      []string{email},
		Subject: renderedSubject,
		Text:    htmlToText(renderedBody),
		HTML:    renderedBody,
	}

	return sendMail(ctx, db, msg)
}

func renderTemplate(name, tpl string, data any) (string, error) {
	t, err := template.New(name).Option("missingkey=zero").Parse(tpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func newVerificationToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func extractIP(addr string) net.IP {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(addr)
}

func htmlToText(html string) string {
	if html == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n\n",
	)
	normalized := replacer.Replace(html)
	var b strings.Builder
	inTag := false
	for _, r := range normalized {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func PasswordChangeHandler(db authDB) http.HandlerFunc {
	type passwordChangeRequest struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		var req passwordChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := validatePassword(req.NewPassword); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var passwordHash string
		err := db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id = $1", u.ID).Scan(&passwordHash)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch user")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to hash new password")
			return
		}

		_, err = db.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2", string(newHash), u.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to update password")
			return
		}

		if err := authmw.RevokeToken(ctx, db, u.TokenID, u.ID, u.TokenExpiresAt, "password_change"); err != nil {
			logging.From(ctx).Error().Err(err).Msg("failed to revoke token after password change")
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func LogoutHandler(db authDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := authmw.FromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := authmw.RevokeToken(ctx, db, u.TokenID, u.ID, u.TokenExpiresAt, "logout"); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to revoke token")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

var newPasswordResetToken = func() (string, string, string, error) {
	selectorRaw := make([]byte, 16)
	if _, err := rand.Read(selectorRaw); err != nil {
		return "", "", "", err
	}
	selector := hex.EncodeToString(selectorRaw)

	validatorRaw := make([]byte, 32)
	if _, err := rand.Read(validatorRaw); err != nil {
		return "", "", "", err
	}
	validator := hex.EncodeToString(validatorRaw)

	return selector, validator, hashToken(validator), nil
}

func RequestPasswordResetHandler(db authDB) http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
	}
	type response struct {
		OK bool `json:"ok"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		allowed, err := isPasswordResetAllowed(ctx, db)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to check if password reset is allowed")
			return
		}
		if !allowed {
			writeJSON(w, http.StatusForbidden, apiError{
				Error:   "reset_disabled",
				Message: "Password reset is currently disabled.",
			})
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusOK, response{OK: true})
			return
		}

		email := strings.TrimSpace(strings.ToLower(req.Email))
		if email == "" {
			writeJSON(w, http.StatusOK, response{OK: true})
			return
		}

		ip := extractIP(r.RemoteAddr).String()

		// Rate limit: check if a token was created in the last 15 minutes for this email or IP
		var count int
		err = db.QueryRow(ctx, `
			SELECT count(*) FROM password_reset_tokens prt
			JOIN users u ON u.id = prt.user_id
			WHERE (u.email = $1 OR prt.ip_address = $2)
			AND prt.created_at > now() - interval '15 minutes'
		`, email, ip).Scan(&count)

		if err == nil && count > 0 {
			logging.From(ctx).Info().Str("email", email).Str("ip", ip).Msg("password reset rate limited")
			writeJSON(w, http.StatusOK, response{OK: true})
			return
		}

		var userID int64
		err = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND is_active = true", email).Scan(&userID)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				logging.From(ctx).Error().Err(err).Msg("failed to look up user for password reset")
			}
			writeJSON(w, http.StatusOK, response{OK: true})
			return
		}

		selector, validator, tokenHash, err := newPasswordResetToken()
		if err != nil {
			logging.From(ctx).Error().Err(err).Msg("failed to generate password reset token")
			writeJSON(w, http.StatusOK, response{OK: true})
			return
		}

		expiresAt := time.Now().Add(time.Hour)
		_, err = db.Exec(ctx, `
			INSERT INTO password_reset_tokens (user_id, selector, token_hash, expires_at, ip_address)
			VALUES ($1, $2, $3, $4, $5)
		`, userID, selector, tokenHash, expiresAt, ip)
		if err != nil {
			logging.From(ctx).Error().Err(err).Msg("failed to store password reset token")
			writeJSON(w, http.StatusOK, response{OK: true})
			return
		}

		instanceURL, _ := getConfigString(ctx, db, "instance.url")
		subjectTpl, _ := getConfigString(ctx, db, "mail.reset_password_subject")
		bodyTpl, _ := getConfigString(ctx, db, "mail.reset_password_body")

		if subjectTpl == "" {
			subjectTpl = "Reset your password"
		}
		if bodyTpl == "" {
			bodyTpl = "Hi,<br><br>You requested a password reset. Please click the link below to set a new password:<br><a href=\"{{.ResetURL}}\">Reset password</a><br><br>If you did not request this, you can safely ignore this email."
		}

		resetURL := strings.TrimRight(instanceURL, "/") + "/reset-password?selector=" + selector + "&token=" + validator

		data := struct {
			ResetURL    string
			Email       string
			InstanceURL string
		}{
			ResetURL:    resetURL,
			Email:       email,
			InstanceURL: instanceURL,
		}

		renderedSubject, err := renderTemplate("reset_subject", subjectTpl, data)
		if err != nil {
			logging.From(ctx).Error().Err(err).Msg("failed to render password reset subject template")
			renderedSubject = subjectTpl // Fallback to raw template if rendering fails
		}

		renderedBody, err := renderTemplate("reset_body", bodyTpl, data)
		if err != nil {
			logging.From(ctx).Error().Err(err).Msg("failed to render password reset body template")
			// If body rendering fails, we don't send the email
		} else {
			if err := sendMail(ctx, db, mail.Message{
				To:      []string{email},
				Subject: renderedSubject,
				HTML:    renderedBody,
				Text:    htmlToText(renderedBody),
			}); err != nil {
				logging.From(ctx).Error().Err(err).Msg("failed to send password reset email")
			}
		}

		writeJSON(w, http.StatusOK, response{OK: true})
	}
}

func ValidatePasswordResetHandler(db authDB) http.HandlerFunc {
	type request struct {
		Selector string `json:"selector"`
		Token    string `json:"token"`
	}
	type response struct {
		Valid     bool       `json:"valid"`
		ExpiresAt *time.Time `json:"expiresAt,omitzero"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		allowed, err := isPasswordResetAllowed(ctx, db)
		if err == nil && !allowed {
			writeJSON(w, http.StatusOK, response{Valid: false})
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusOK, response{Valid: false})
			return
		}

		var tokenHash string
		var expiresAt time.Time
		err = db.QueryRow(ctx, "SELECT token_hash, expires_at FROM password_reset_tokens WHERE selector = $1 AND expires_at > now()", req.Selector).Scan(&tokenHash, &expiresAt)
		if err != nil {
			writeJSON(w, http.StatusOK, response{Valid: false})
			return
		}

		if hashToken(req.Token) != tokenHash {
			writeJSON(w, http.StatusOK, response{Valid: false})
			return
		}

		writeJSON(w, http.StatusOK, response{Valid: true, ExpiresAt: &expiresAt})
	}
}

func ConfirmPasswordResetHandler(db authDB) http.HandlerFunc {
	type request struct {
		Selector    string `json:"selector"`
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	type response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		allowed, err := isPasswordResetAllowed(ctx, db)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: "internal_error"})
			return
		}
		if !allowed {
			writeJSON(w, http.StatusForbidden, response{OK: false, Error: "reset_disabled"})
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid_request"})
			return
		}

		if err := validatePassword(req.NewPassword); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "password_too_weak"})
			return
		}

		var userID int64
		var tokenHash string
		err = db.QueryRow(ctx, "SELECT user_id, token_hash FROM password_reset_tokens WHERE selector = $1 AND expires_at > now()", req.Selector).Scan(&userID, &tokenHash)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid_or_expired"})
			return
		}

		if hashToken(req.Token) != tokenHash {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid_or_expired"})
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: "internal_error"})
			return
		}

		_, err = db.Exec(ctx, "UPDATE users SET password_hash = $1, token_version = token_version + 1, updated_at = now() WHERE id = $2", string(hashedPassword), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: "internal_error"})
			return
		}

		_, _ = db.Exec(ctx, "DELETE FROM password_reset_tokens WHERE selector = $1", req.Selector)

		writeJSON(w, http.StatusOK, response{OK: true})
	}
}
