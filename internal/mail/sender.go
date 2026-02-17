package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"GoToDo/internal/logging"
	"GoToDo/internal/secrets"
	"github.com/jackc/pgx/v5"
)

var ErrDisabled = errors.New("mail is disabled")

type Config struct {
	Enabled     bool
	FromName    string
	FromAddress string
	SMTP        SMTPConfig
}

type SMTPConfig struct {
	Host     string
	Port     int
	TLSMode  string
	Username string
	Password string
}

type Message struct {
	To      []string
	Subject string
	Text    string
	HTML    string
}

type Sender struct {
	cfg Config
}

type configQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func NewSender(cfg Config) *Sender {
	return &Sender{cfg: cfg}
}

func NewSenderFromDB(ctx context.Context, db configQuerier) (*Sender, error) {
	cfg, err := LoadConfig(ctx, db)
	if err != nil {
		return nil, err
	}
	return &Sender{cfg: cfg}, nil
}

func Send(ctx context.Context, db configQuerier, msg Message) error {
	sender, err := NewSenderFromDB(ctx, db)
	if err != nil {
		return err
	}
	return sender.Send(ctx, msg)
}

func (s *Sender) Send(ctx context.Context, msg Message) error {
	l := logging.From(ctx)
	if !s.cfg.Enabled {
		l.Debug().Msg("mail sending skipped: disabled")
		return ErrDisabled
	}
	if s.cfg.FromAddress == "" {
		return errors.New("mail.fromAddress is not configured")
	}
	if s.cfg.SMTP.Host == "" {
		return errors.New("mail.smtp.host is not configured")
	}
	if s.cfg.SMTP.Port == 0 {
		return errors.New("mail.smtp.port is not configured")
	}

	l.Info().
		Strs("to", msg.To).
		Str("subject", msg.Subject).
		Msg("sending email")

	payload, err := buildMessage(s.cfg, msg)
	if err != nil {
		l.Error().Err(err).Msg("failed to build email message")
		return err
	}

	err = s.sendSMTP(ctx, msg.To, payload)
	if err != nil {
		l.Error().Err(err).Msg("failed to send email via smtp")
		return err
	}
	return nil
}

func (s *Sender) sendSMTP(ctx context.Context, recipients []string, payload []byte) error {
	l := logging.From(ctx)
	if len(recipients) == 0 {
		return errors.New("missing recipients")
	}

	mode := strings.ToLower(strings.TrimSpace(s.cfg.SMTP.TLSMode))
	if mode == "" {
		mode = "starttls"
	}

	addr := net.JoinHostPort(s.cfg.SMTP.Host, strconv.Itoa(s.cfg.SMTP.Port))
	l.Debug().
		Str("addr", addr).
		Str("mode", mode).
		Msg("connecting to smtp server")

	dialer := net.Dialer{Timeout: 10 * time.Second}

	var conn net.Conn
	var err error

	if mode == "tls" {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: s.cfg.SMTP.Host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return err
		}
		conn = tlsConn
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
	}

	client, err := smtp.NewClient(conn, s.cfg.SMTP.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if mode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.SMTP.Host}); err != nil {
			return err
		}
	}

	if s.cfg.SMTP.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(s.cfg.FromAddress); err != nil {
		return err
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	l.Debug().Msg("email sent successfully")
	return client.Quit()
}

func LoadConfig(ctx context.Context, db configQuerier) (Config, error) {
	keys := []string{
		"mail.enabled",
		"mail.fromName",
		"mail.fromAddress",
		"mail.smtp.host",
		"mail.smtp.port",
		"mail.smtp.tls_mode",
		"mail.smtp.username",
		"mail.smtp.password",
	}

	rows, err := db.Query(ctx, `
		SELECT key, value_json, is_secret
		FROM config_keys
		WHERE key = ANY($1)
	`, keys)
	if err != nil {
		return Config{}, err
	}
	defer rows.Close()

	type rawValue struct {
		value    []byte
		isSecret bool
	}
	values := make(map[string]rawValue, len(keys))
	for rows.Next() {
		var key string
		var raw []byte
		var isSecret bool
		if err := rows.Scan(&key, &raw, &isSecret); err != nil {
			return Config{}, err
		}
		values[key] = rawValue{value: raw, isSecret: isSecret}
	}
	if err := rows.Err(); err != nil {
		return Config{}, err
	}

	get := func(key string) (rawValue, error) {
		val, ok := values[key]
		if !ok {
			return rawValue{}, fmt.Errorf("missing config key %s", key)
		}
		return val, nil
	}

	enabledRaw, err := get("mail.enabled")
	if err != nil {
		return Config{}, err
	}
	enabled, err := decodeBool(enabledRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.enabled: %w", err)
	}

	fromNameRaw, err := get("mail.fromName")
	if err != nil {
		return Config{}, err
	}
	fromName, err := decodeString(fromNameRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.fromName: %w", err)
	}

	fromAddrRaw, err := get("mail.fromAddress")
	if err != nil {
		return Config{}, err
	}
	fromAddress, err := decodeString(fromAddrRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.fromAddress: %w", err)
	}

	hostRaw, err := get("mail.smtp.host")
	if err != nil {
		return Config{}, err
	}
	host, err := decodeString(hostRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.smtp.host: %w", err)
	}

	portRaw, err := get("mail.smtp.port")
	if err != nil {
		return Config{}, err
	}
	port, err := decodeInt(portRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.smtp.port: %w", err)
	}

	tlsModeRaw, err := get("mail.smtp.tls_mode")
	if err != nil {
		return Config{}, err
	}
	tlsMode, err := decodeString(tlsModeRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.smtp.tls_mode: %w", err)
	}

	userRaw, err := get("mail.smtp.username")
	if err != nil {
		return Config{}, err
	}
	username, err := decodeString(userRaw.value)
	if err != nil {
		return Config{}, fmt.Errorf("mail.smtp.username: %w", err)
	}

	passRaw, err := get("mail.smtp.password")
	if err != nil {
		return Config{}, err
	}
	password, err := decodeSecret("mail.smtp.password", passRaw.value, passRaw.isSecret)
	if err != nil {
		return Config{}, fmt.Errorf("mail.smtp.password: %w", err)
	}

	return Config{
		Enabled:     enabled,
		FromName:    fromName,
		FromAddress: fromAddress,
		SMTP: SMTPConfig{
			Host:     host,
			Port:     port,
			TLSMode:  tlsMode,
			Username: username,
			Password: password,
		},
	}, nil
}

func decodeBool(raw []byte) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, err
	}
	return v, nil
}

func decodeInt(raw []byte) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func decodeString(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	return v, nil
}

func decodeSecret(key string, raw []byte, isSecret bool) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	val, err := decodeString(raw)
	if err != nil {
		return "", err
	}
	if val == "" || !isSecret || !strings.HasPrefix(val, "enc:v1:") {
		return val, nil
	}

	mk, err := secrets.LoadMasterKey()
	if err != nil {
		return "", err
	}
	plaintext, err := secrets.DecryptString(val, mk, []byte(key))
	if err == nil {
		return plaintext, nil
	}
	plaintext, errFallback := secrets.DecryptString(val, mk, []byte("config_key:"+key))
	if errFallback == nil {
		return plaintext, nil
	}
	return "", err
}

func buildMessage(cfg Config, msg Message) ([]byte, error) {
	if len(msg.To) == 0 {
		return nil, errors.New("missing recipients")
	}
	if strings.TrimSpace(msg.Text) == "" && strings.TrimSpace(msg.HTML) == "" {
		return nil, errors.New("missing message body")
	}

	fromHeader := cfg.FromAddress
	if cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", cfg.FromName), cfg.FromAddress)
	}

	subject := strings.TrimSpace(msg.Subject)
	if subject != "" {
		subject = mime.QEncoding.Encode("utf-8", subject)
	}

	var buf bytes.Buffer
	buf.WriteString("From: " + fromHeader + "\r\n")
	buf.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	if subject != "" {
		buf.WriteString("Subject: " + subject + "\r\n")
	}
	buf.WriteString("MIME-Version: 1.0\r\n")

	if msg.Text != "" && msg.HTML != "" {
		boundary := randomBoundary()
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n", boundary))
		buf.WriteString("\r\n")
		writePart(&buf, boundary, "text/plain; charset=UTF-8", msg.Text)
		writePart(&buf, boundary, "text/html; charset=UTF-8", msg.HTML)
		buf.WriteString("--" + boundary + "--\r\n")
		return buf.Bytes(), nil
	}

	body := msg.Text
	contentType := "text/plain; charset=UTF-8"
	if msg.HTML != "" {
		body = msg.HTML
		contentType = "text/html; charset=UTF-8"
	}

	buf.WriteString("Content-Type: " + contentType + "\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&buf)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
	buf.WriteString("\r\n")

	return buf.Bytes(), nil
}

func writePart(buf *bytes.Buffer, boundary, contentType, body string) {
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: " + contentType + "\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	qp := quotedprintable.NewWriter(buf)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
	buf.WriteString("\r\n")
}

func randomBoundary() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return fmt.Sprintf("%x", b)
}
