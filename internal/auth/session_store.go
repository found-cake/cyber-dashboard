package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

const sessionSchema = `
CREATE TABLE IF NOT EXISTS refresh_sessions (
    session_id TEXT PRIMARY KEY,
    refresh_token_hash BLOB NOT NULL,
    token_version INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS refresh_sessions_expires_at_idx ON refresh_sessions(expires_at);`

type refreshSession struct {
	id        string
	tokenHash [sha256.Size]byte
	version   uint64
	expiresAt time.Time
}

type SessionStore struct {
	db  *sql.DB
	now func() time.Time
}

func OpenSessionStore(ctx context.Context, path string, now func() time.Time) (*SessionStore, error) {
	if now == nil {
		now = time.Now
	}
	db, err := sql.Open("sqlite", sessionDataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("open refresh session database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	store := &SessionStore{db: db, now: now}
	if _, err := db.ExecContext(ctx, sessionSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize refresh session database: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure refresh session database: %w", err)
	}
	if err := store.deleteExpired(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SessionStore) Close() error {
	return s.db.Close()
}

func (s *SessionStore) Create(ctx context.Context, session refreshSession) error {
	now := s.now().UTC().Unix()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO refresh_sessions
    (session_id, refresh_token_hash, token_version, expires_at, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?)`, session.id, session.tokenHash[:], session.version, session.expiresAt.UTC().Unix(), now, now)
	if err != nil {
		return fmt.Errorf("create refresh session: %w", err)
	}
	return nil
}

func (s *SessionStore) Rotate(ctx context.Context, currentHash [sha256.Size]byte, next refreshSession) (rotated bool, rotateErr error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin refresh session rotation: %w", err)
	}
	finished := false
	defer func() {
		if finished {
			return
		}
		if err := tx.Rollback(); err != nil {
			rotateErr = errors.Join(rotateErr, fmt.Errorf("rollback refresh session rotation: %w", err))
		}
	}()
	now := s.now().UTC().Unix()
	result, err := tx.ExecContext(ctx, `
UPDATE refresh_sessions
SET refresh_token_hash = ?, expires_at = ?, last_used_at = ?
WHERE session_id = ?
  AND refresh_token_hash = ?
  AND token_version = ?
  AND expires_at > ?`, next.tokenHash[:], next.expiresAt.UTC().Unix(), now, next.id, currentHash[:], next.version, now)
	if err != nil {
		return false, fmt.Errorf("rotate refresh session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read refresh rotation result: %w", err)
	}
	if rows != 1 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM refresh_sessions WHERE expires_at <= ?`, now); err != nil {
		return false, fmt.Errorf("delete expired refresh sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit refresh session rotation: %w", err)
	}
	finished = true
	return true, nil
}

func (s *SessionStore) ReplaceAll(ctx context.Context, session refreshSession) (replaceErr error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin refresh session replacement: %w", err)
	}
	defer func() {
		if replaceErr != nil {
			replaceErr = errors.Join(replaceErr, tx.Rollback())
		}
	}()
	if _, err := tx.ExecContext(ctx, `DELETE FROM refresh_sessions`); err != nil {
		return fmt.Errorf("clear refresh sessions: %w", err)
	}
	now := s.now().UTC().Unix()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO refresh_sessions
    (session_id, refresh_token_hash, token_version, expires_at, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?)`, session.id, session.tokenHash[:], session.version, session.expiresAt.UTC().Unix(), now, now); err != nil {
		return fmt.Errorf("replace refresh session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit refresh session replacement: %w", err)
	}
	return nil
}

func (s *SessionStore) Revoke(ctx context.Context, sessionID string, tokenHash [sha256.Size]byte) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM refresh_sessions WHERE session_id = ? AND refresh_token_hash = ?`, sessionID, tokenHash[:]); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	return nil
}

func (s *SessionStore) deleteExpired(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM refresh_sessions WHERE expires_at <= ?`, s.now().UTC().Unix()); err != nil {
		return fmt.Errorf("delete expired refresh sessions: %w", err)
	}
	return nil
}

func sessionDataSourceName(path string) string {
	return (&url.URL{
		Scheme: "file",
		Opaque: (&url.URL{Path: path}).EscapedPath(),
		RawQuery: url.Values{"_pragma": {
			"busy_timeout(5000)", "journal_mode(WAL)", "synchronous(FULL)",
		}}.Encode(),
	}).String()
}
