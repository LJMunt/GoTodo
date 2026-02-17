package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

const prefixV1 = "enc:v1:"

func LoadMasterKey() ([]byte, error) {
	if p := os.Getenv("APP_MASTER_KEY_FILE"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		keyB64 := strings.TrimSpace(string(b))
		return base64.StdEncoding.DecodeString(keyB64)
	}
	if b64 := os.Getenv("SECRETS_MASTER_KEY_B64"); b64 != "" {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	}
	return nil, errors.New("no master key configured (SECRET_MASTER_KEY_FILE or SECRETS_MASTER_KEY_B64)")
}

func EncryptString(plaintext string, key32 []byte, aad []byte) (string, error) {
	if len(key32) != 32 {
		return "", errors.New("master key must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key32)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ct := gcm.Seal(nil, nonce, []byte(plaintext), aad)

	blob := make([]byte, 0, len(nonce)+len(ct))
	blob = append(blob, nonce...)
	blob = append(blob, ct...)

	return prefixV1 + base64.StdEncoding.EncodeToString(blob), nil
}

func DecryptString(ciphertext string, key32 []byte, aad []byte) (string, error) {
	if !strings.HasPrefix(ciphertext, prefixV1) {
		return "", errors.New("not an encrypted secret (missing enc:v1: prefix)")
	}
	b64 := strings.TrimPrefix(ciphertext, prefixV1)

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	if len(key32) != 32 {
		return "", errors.New("master key must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key32)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}

	nonce := raw[:ns]
	ct := raw[ns:]

	pt, err := gcm.Open(nil, nonce, ct, aad)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
