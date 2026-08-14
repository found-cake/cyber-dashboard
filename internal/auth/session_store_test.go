package auth

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStoreRotatesOneCurrentTokenAndPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "dashboard.sessions.db")
	store, err := OpenSessionStore(ctx, path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	currentHash := sha256.Sum256([]byte("current-refresh-token"))
	nextHash := sha256.Sum256([]byte("next-refresh-token"))
	if err := store.Create(ctx, refreshSession{
		id: "session-one", tokenHash: currentHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	}); err != nil {
		t.Fatal(err)
	}

	rotated, err := store.Rotate(ctx, currentHash, refreshSession{
		id: "session-one", tokenHash: nextHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	})
	if err != nil || !rotated {
		t.Fatalf("first rotation = %v, %v", rotated, err)
	}
	replayed, err := store.Rotate(ctx, currentHash, refreshSession{
		id: "session-one", tokenHash: currentHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	})
	if err != nil || replayed {
		t.Fatalf("replayed rotation = %v, %v", replayed, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenSessionStore(ctx, path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rotated, err = store.Rotate(ctx, nextHash, refreshSession{
		id: "session-one", tokenHash: currentHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	})
	if err != nil || !rotated {
		t.Fatalf("rotation after reopen = %v, %v", rotated, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session database mode = %o, want 600", info.Mode().Perm())
	}
}

func TestSessionStoreRotateDeletesExpiredSessions(t *testing.T) {
	// Given
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	store, err := OpenSessionStore(ctx, filepath.Join(t.TempDir(), "dashboard.sessions.db"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	currentHash := sha256.Sum256([]byte("current-refresh-token"))
	nextHash := sha256.Sum256([]byte("next-refresh-token"))
	if err := store.Create(ctx, refreshSession{
		id: "active-session", tokenHash: currentHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, refreshSession{
		id: "expired-session", tokenHash: currentHash, version: 1, expiresAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	// When
	rotated, err := store.Rotate(ctx, currentHash, refreshSession{
		id: "active-session", tokenHash: nextHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	})

	// Then
	if err != nil || !rotated {
		t.Fatalf("rotation = %v, %v", rotated, err)
	}
	var expiredSessions int
	if err := store.db.QueryRowContext(ctx,
		`SELECT count(*) FROM refresh_sessions WHERE expires_at <= ?`, now.Unix()).Scan(&expiredSessions); err != nil {
		t.Fatal(err)
	}
	if expiredSessions != 0 {
		t.Fatalf("expired sessions after rotation = %d, want 0", expiredSessions)
	}
}

func TestSessionStoreRotateKeepsExpiredSessionsWhenTokenIsRejected(t *testing.T) {
	// Given
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	store, err := OpenSessionStore(ctx, filepath.Join(t.TempDir(), "dashboard.sessions.db"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	currentHash := sha256.Sum256([]byte("current-refresh-token"))
	if err := store.Create(ctx, refreshSession{
		id: "active-session", tokenHash: currentHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, refreshSession{
		id: "expired-session", tokenHash: currentHash, version: 1, expiresAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	// When
	rotated, err := store.Rotate(ctx, sha256.Sum256([]byte("replayed-refresh-token")), refreshSession{
		id: "active-session", tokenHash: currentHash, version: 1, expiresAt: now.Add(RefreshLifetime),
	})

	// Then
	if err != nil || rotated {
		t.Fatalf("rejected rotation = %v, %v", rotated, err)
	}
	var expiredSessions int
	if err := store.db.QueryRowContext(ctx,
		`SELECT count(*) FROM refresh_sessions WHERE expires_at <= ?`, now.Unix()).Scan(&expiredSessions); err != nil {
		t.Fatal(err)
	}
	if expiredSessions != 1 {
		t.Fatalf("expired sessions after rejected rotation = %d, want 1", expiredSessions)
	}
}
