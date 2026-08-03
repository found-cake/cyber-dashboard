package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const secretPrefix = "v1:"

type secretBox struct {
	aead cipher.AEAD
}

func openSecretBox(path string) (*secretBox, error) {
	key, err := readOrCreateKey(path)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret GCM: %w", err)
	}
	return &secretBox{aead: aead}, nil
}

func readOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("secret key has invalid length")
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read secret key: %w", err)
	}
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create secret key: %w", err)
	}
	if _, err := file.Write(key); err != nil {
		return nil, errors.Join(fmt.Errorf("write secret key: %w", err), file.Close())
	}
	if err := file.Sync(); err != nil {
		return nil, errors.Join(fmt.Errorf("sync secret key: %w", err), file.Close())
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close secret key: %w", err)
	}
	return key, nil
}

func (b *secretBox) seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (b *secretBox) open(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, secretPrefix) {
		return "", fmt.Errorf("encrypted secret has unsupported version")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, secretPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}
	nonceSize := b.aead.NonceSize()
	if len(sealed) < nonceSize {
		return "", fmt.Errorf("encrypted secret is truncated")
	}
	plaintext, err := b.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}
