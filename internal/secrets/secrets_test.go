package secrets

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustKey32(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		key[i] = byte(i + 1)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := mustKey32(t)
	aad := []byte("config_key:mail.smtp.password")

	plaintext := "super-secret-smtp-password"
	ciphertext, err := EncryptString(plaintext, key, aad)
	if err != nil {
		t.Fatalf("EncryptString() error: %v", err)
	}
	if !strings.HasPrefix(ciphertext, prefixV1) {
		t.Fatalf("expected ciphertext to have prefix %q, got %q", prefixV1, ciphertext)
	}
	if ciphertext == plaintext {
		t.Fatalf("ciphertext should not equal plaintext")
	}

	got, err := DecryptString(ciphertext, key, aad)
	if err != nil {
		t.Fatalf("DecryptString() error: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestEncryptString_KeyLengthValidation(t *testing.T) {
	aad := []byte("config_key:x")

	_, err := EncryptString("x", []byte("too-short"), aad)
	if err == nil {
		t.Fatalf("expected error for short key")
	}

	_, err = EncryptString("x", make([]byte, 31), aad)
	if err == nil {
		t.Fatalf("expected error for 31-byte key")
	}

	_, err = EncryptString("x", make([]byte, 33), aad)
	if err == nil {
		t.Fatalf("expected error for 33-byte key")
	}
}

func TestDecryptString_RejectsMissingPrefix(t *testing.T) {
	key := mustKey32(t)
	aad := []byte("config_key:x")

	_, err := DecryptString("not-encrypted", key, aad)
	if err == nil {
		t.Fatalf("expected error for missing prefix")
	}
}

func TestDecryptString_AADMismatchFails(t *testing.T) {
	key := mustKey32(t)
	ciphertext, err := EncryptString("pw", key, []byte("config_key:a"))
	if err != nil {
		t.Fatalf("EncryptString() error: %v", err)
	}

	_, err = DecryptString(ciphertext, key, []byte("config_key:b"))
	if err == nil {
		t.Fatalf("expected error when AAD differs")
	}
}

func TestDecryptString_TamperFails(t *testing.T) {
	key := mustKey32(t)
	aad := []byte("config_key:x")

	ciphertext, err := EncryptString("pw", key, aad)
	if err != nil {
		t.Fatalf("EncryptString() error: %v", err)
	}
	if !strings.HasPrefix(ciphertext, prefixV1) {
		t.Fatalf("missing expected prefix")
	}

	// Decode, flip a bit, re-encode
	rawB64 := strings.TrimPrefix(ciphertext, prefixV1)
	raw, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("decoded ciphertext unexpectedly empty")
	}
	raw[len(raw)-1] ^= 0x01 // flip last bit
	tampered := prefixV1 + base64.StdEncoding.EncodeToString(raw)

	_, err = DecryptString(tampered, key, aad)
	if err == nil {
		t.Fatalf("expected error for tampered ciphertext")
	}
}

func TestDecryptString_WrongKeyFails(t *testing.T) {
	key := mustKey32(t)
	otherKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		otherKey[i] = byte(255 - i)
	}
	aad := []byte("config_key:x")

	ciphertext, err := EncryptString("pw", key, aad)
	if err != nil {
		t.Fatalf("EncryptString() error: %v", err)
	}

	_, err = DecryptString(ciphertext, otherKey, aad)
	if err == nil {
		t.Fatalf("expected error for wrong key")
	}
}

func TestLoadMasterKey_FromFile(t *testing.T) {
	// Ensure env isolation
	oldFile := os.Getenv("APP_MASTER_KEY_FILE")
	oldB64 := os.Getenv("SECRETS_MASTER_KEY_B64")
	t.Cleanup(func() {
		_ = os.Setenv("APP_MASTER_KEY_FILE", oldFile)
		_ = os.Setenv("SECRETS_MASTER_KEY_B64", oldB64)
	})

	// Create key and write to file (base64 string), like Docker/K8s secret file
	key := mustKey32(t)
	keyB64 := base64.StdEncoding.EncodeToString(key)

	dir := t.TempDir()
	path := filepath.Join(dir, "app_master_key")
	if err := os.WriteFile(path, []byte(keyB64+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	_ = os.Unsetenv("SECRETS_MASTER_KEY_B64")
	if err := os.Setenv("APP_MASTER_KEY_FILE", path); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}

	got, err := LoadMasterKey()
	if err != nil {
		t.Fatalf("LoadMasterKey() error: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(got))
	}
	if string(got) != string(key) {
		t.Fatalf("file-loaded key mismatch")
	}
}

func TestLoadMasterKey_FromEnvFallback(t *testing.T) {
	// Ensure env isolation
	oldFile := os.Getenv("APP_MASTER_KEY_FILE")
	oldB64 := os.Getenv("SECRETS_MASTER_KEY_B64")
	t.Cleanup(func() {
		_ = os.Setenv("APP_MASTER_KEY_FILE", oldFile)
		_ = os.Setenv("SECRETS_MASTER_KEY_B64", oldB64)
	})

	_ = os.Unsetenv("APP_MASTER_KEY_FILE")

	key := mustKey32(t)
	keyB64 := base64.StdEncoding.EncodeToString(key)
	if err := os.Setenv("SECRETS_MASTER_KEY_B64", keyB64); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}

	got, err := LoadMasterKey()
	if err != nil {
		t.Fatalf("LoadMasterKey() error: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(got))
	}
	if string(got) != string(key) {
		t.Fatalf("env-loaded key mismatch")
	}
}

func TestLoadMasterKey_NoKeyConfigured(t *testing.T) {
	oldFile := os.Getenv("APP_MASTER_KEY_FILE")
	oldB64 := os.Getenv("SECRETS_MASTER_KEY_B64")
	t.Cleanup(func() {
		_ = os.Setenv("APP_MASTER_KEY_FILE", oldFile)
		_ = os.Setenv("SECRETS_MASTER_KEY_B64", oldB64)
	})

	_ = os.Unsetenv("APP_MASTER_KEY_FILE")
	_ = os.Unsetenv("SECRETS_MASTER_KEY_B64")

	_, err := LoadMasterKey()
	if err == nil {
		t.Fatalf("expected error when no master key is configured")
	}
}
