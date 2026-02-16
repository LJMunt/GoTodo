package mail

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"GoToDo/internal/secrets"
)

func mustKey32(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		key[i] = byte(i + 1)
	}
	return key
}

func withMasterKey(t *testing.T, key []byte) {
	t.Helper()
	oldFile := os.Getenv("APP_MASTER_KEY_FILE")
	oldB64 := os.Getenv("SECRETS_MASTER_KEY_B64")
	t.Cleanup(func() {
		_ = os.Setenv("APP_MASTER_KEY_FILE", oldFile)
		_ = os.Setenv("SECRETS_MASTER_KEY_B64", oldB64)
	})
	_ = os.Unsetenv("APP_MASTER_KEY_FILE")
	if err := os.Setenv("SECRETS_MASTER_KEY_B64", base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}
}

func TestSenderSend_Disabled(t *testing.T) {
	sender := NewSender(Config{Enabled: false})
	if err := sender.Send(t.Context(), Message{To: []string{"a@example.com"}, Text: "hi"}); err != ErrDisabled {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestBuildMessage_BothBodies(t *testing.T) {
	cfg := Config{FromName: "GoToDo", FromAddress: "support@example.com"}
	msg := Message{
		To:      []string{"user@example.com"},
		Subject: "Welcome",
		Text:    "Hello text",
		HTML:    "<p>Hello html</p>",
	}

	out, err := buildMessage(cfg, msg)
	if err != nil {
		t.Fatalf("buildMessage error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "multipart/alternative") {
		t.Fatalf("expected multipart/alternative, got %q", got)
	}
	if !strings.Contains(got, "Hello text") || !strings.Contains(got, "Hello html") {
		t.Fatalf("expected both bodies to be present")
	}
}

func TestBuildMessage_MissingBody(t *testing.T) {
	cfg := Config{FromAddress: "support@example.com"}
	msg := Message{To: []string{"user@example.com"}}
	if _, err := buildMessage(cfg, msg); err == nil {
		t.Fatalf("expected error for missing body")
	}
}

func TestDecodeSecret_FallbackAAD(t *testing.T) {
	key := mustKey32(t)
	withMasterKey(t, key)

	ciphertext, err := secrets.EncryptString("pw", key, []byte("config_key:mail.smtp.password"))
	if err != nil {
		t.Fatalf("EncryptString error: %v", err)
	}
	raw, err := json.Marshal(ciphertext)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	got, err := decodeSecret("mail.smtp.password", raw, true)
	if err != nil {
		t.Fatalf("decodeSecret error: %v", err)
	}
	if got != "pw" {
		t.Fatalf("expected decrypted secret, got %q", got)
	}
}
