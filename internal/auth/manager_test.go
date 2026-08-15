package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestManagerPasswordLifecycleAndJWTExpiry(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(directory, "dashboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(db)

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	key := []byte("0123456789abcdef0123456789abcdef")
	sessionPath := filepath.Join(directory, "dashboard.sessions.db")
	sessionStore, err := OpenSessionStore(ctx, sessionPath, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sessionStore.Close() }()
	manager, err := NewManager(db, sessionStore, key)
	if err != nil {
		t.Fatal(err)
	}
	password, generated, err := manager.EnsurePassword(ctx)
	if err != nil || !generated || len(password) < 20 {
		t.Fatalf("EnsurePassword() = %q, %v, %v", password, generated, err)
	}
	if _, generated, err = manager.EnsurePassword(ctx); err != nil || generated {
		t.Fatalf("second EnsurePassword() generated = %v, err = %v", generated, err)
	}

	pair, err := manager.Login(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.VerifyAccess(ctx, pair.AccessToken); err != nil {
		t.Fatalf("fresh access token: %v", err)
	}
	if err := sessionStore.Close(); err != nil {
		t.Fatal(err)
	}
	sessionStore, err = OpenSessionStore(ctx, sessionPath, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	restartedManager, err := NewManager(db, sessionStore, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedManager.VerifyAccess(ctx, pair.AccessToken); err != nil {
		t.Fatalf("access token after manager restart: %v", err)
	}
	manager = restartedManager
	now = now.Add(15*time.Minute + time.Second)
	if err := manager.VerifyAccess(ctx, pair.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired access token error = %v", err)
	}
	refreshed, err := manager.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh after access expiry: %v", err)
	}
	if _, err := manager.Refresh(ctx, pair.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("replayed refresh token error = %v", err)
	}
	if err := manager.VerifyAccess(ctx, refreshed.AccessToken); err != nil {
		t.Fatalf("refreshed access token: %v", err)
	}

	changed, err := manager.ChangePassword(ctx, password, "a-new-local-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.VerifyAccess(ctx, refreshed.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old token after password change error = %v", err)
	}
	if _, err := manager.Refresh(ctx, refreshed.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old refresh token after password change error = %v", err)
	}
	if err := manager.VerifyAccess(ctx, changed.AccessToken); err != nil {
		t.Fatalf("new access token: %v", err)
	}
	changed, err = manager.Refresh(ctx, changed.RefreshToken)
	if err != nil {
		t.Fatalf("new refresh token: %v", err)
	}
	if _, err := manager.Login(ctx, password); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password login error = %v", err)
	}
	if _, err := manager.Login(ctx, "a-new-local-password"); err != nil {
		t.Fatalf("new password login: %v", err)
	}

	now = now.Add(3*24*time.Hour + time.Minute)
	if _, err := manager.Refresh(ctx, changed.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired refresh token error = %v", err)
	}
}

func TestPasswordValidation(t *testing.T) {
	if _, err := hashPassword("short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("short password error = %v", err)
	}
	hash, err := hashPassword("a-valid-local-password")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "a-valid-local-password") || verifyPassword(hash, "wrong-password-value") {
		t.Fatal("password verification returned the wrong result")
	}
}

func TestManagerChangePasswordAllowsOneWinnerWhenRequestsConcurrent(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(directory, "dashboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(db)
	sessionStore, err := OpenSessionStore(ctx, filepath.Join(directory, "dashboard.sessions.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sessionStore.Close()
	manager, err := NewManager(db, sessionStore, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	currentPassword, _, err := manager.EnsurePassword(ctx)
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		password string
		pair     TokenPair
		err      error
	}
	replacements := []string{
		"replacement-password-one",
		"replacement-password-two",
		"replacement-password-three",
		"replacement-password-four",
	}
	start := make(chan struct{})
	results := make(chan result, len(replacements))
	for _, replacement := range replacements {
		go func() {
			<-start
			pair, changeErr := manager.ChangePassword(ctx, currentPassword, replacement)
			results <- result{password: replacement, pair: pair, err: changeErr}
		}()
	}

	close(start)
	winners := make([]result, 0, 1)
	for range replacements {
		change := <-results
		if change.err == nil {
			winners = append(winners, change)
			continue
		}
		if !errors.Is(change.err, ErrInvalidCredentials) {
			t.Fatalf("concurrent password change error = %v", change.err)
		}
	}

	if len(winners) != 1 {
		t.Fatalf("concurrent password changes produced %d winners, want 1", len(winners))
	}
	if err := manager.VerifyAccess(ctx, winners[0].pair.AccessToken); err != nil {
		t.Fatalf("winning access token: %v", err)
	}
	if _, err := manager.Login(ctx, winners[0].password); err != nil {
		t.Fatalf("winning password login: %v", err)
	}
}
