package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
)

const signingKeyLength = 32

func LoadOrCreateSigningKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != signingKeyLength {
			return nil, fmt.Errorf("JWT signing key must be %d bytes", signingKeyLength)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read JWT signing key: %w", err)
	}
	key = make([]byte, signingKeyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate JWT signing key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return LoadOrCreateSigningKey(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create JWT signing key: %w", err)
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write JWT signing key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close JWT signing key: %w", err)
	}
	return key, nil
}
