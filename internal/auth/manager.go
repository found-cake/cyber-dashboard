package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	AccessLifetime  = 30 * time.Minute
	RefreshLifetime = 3 * 24 * time.Hour
	tokenIssuer     = "cyber-dashboard"
	tokenAudience   = "cyber-dashboard-web"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type Manager struct {
	db       *gorm.DB
	sessions *SessionStore
	key      []byte
	now      func() time.Time
}

type tokenClaims struct {
	Kind      string `json:"kind"`
	Version   uint64 `json:"ver"`
	SessionID string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

func NewManager(db *gorm.DB, sessions *SessionStore, signingKey []byte) (*Manager, error) {
	if db == nil || sessions == nil || len(signingKey) < 32 {
		return nil, fmt.Errorf("auth manager requires credential and session databases plus a 32-byte signing key")
	}
	key := append([]byte(nil), signingKey...)
	return &Manager{db: db, sessions: sessions, key: key, now: sessions.now}, nil
}

func (m *Manager) EnsurePassword(ctx context.Context) (string, bool, error) {
	var credential database.AdminCredential
	err := m.db.WithContext(ctx).First(&credential, 1).Error
	if err == nil {
		return "", false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, fmt.Errorf("read admin credential: %w", err)
	}
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", false, fmt.Errorf("generate initial password: %w", err)
	}
	password := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := hashPassword(password)
	if err != nil {
		return "", false, err
	}
	credential = database.AdminCredential{ID: 1, PasswordHash: hash, TokenVersion: 1}
	if err := m.db.WithContext(ctx).Create(&credential).Error; err != nil {
		return "", false, fmt.Errorf("save admin credential: %w", err)
	}
	return password, true, nil
}

func (m *Manager) Login(ctx context.Context, password string) (TokenPair, error) {
	credential, err := m.credential(ctx)
	if err != nil {
		return TokenPair{}, err
	}
	if !verifyPassword(credential.PasswordHash, password) {
		return TokenPair{}, ErrInvalidCredentials
	}
	return m.newSessionPair(ctx, credential.TokenVersion)
}

func (m *Manager) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := m.parse(refreshToken, "refresh")
	if err != nil {
		return TokenPair{}, err
	}
	if err := m.verifyVersion(ctx, claims.Version); err != nil {
		return TokenPair{}, err
	}
	pair, session, err := m.prepareSessionPair(claims.Version, claims.SessionID)
	if err != nil {
		return TokenPair{}, err
	}
	rotated, err := m.sessions.Rotate(ctx, sha256.Sum256([]byte(refreshToken)), session)
	if err != nil {
		return TokenPair{}, err
	}
	if !rotated {
		return TokenPair{}, ErrInvalidToken
	}
	return pair, nil
}

func (m *Manager) Logout(ctx context.Context, refreshToken string) error {
	claims, err := m.parse(refreshToken, "refresh")
	if err != nil {
		return nil
	}
	return m.sessions.Revoke(ctx, claims.SessionID, sha256.Sum256([]byte(refreshToken)))
}

func (m *Manager) VerifyAccess(ctx context.Context, accessToken string) error {
	claims, err := m.parse(accessToken, "access")
	if err != nil {
		return err
	}
	return m.verifyVersion(ctx, claims.Version)
}

func (m *Manager) ChangePassword(ctx context.Context, currentPassword, newPassword string) (TokenPair, error) {
	credential, err := m.credential(ctx)
	if err != nil {
		return TokenPair{}, err
	}
	if !verifyPassword(credential.PasswordHash, currentPassword) {
		return TokenPair{}, ErrInvalidCredentials
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return TokenPair{}, err
	}
	nextVersion := credential.TokenVersion + 1
	result := m.db.WithContext(ctx).Model(&database.AdminCredential{}).
		Where("id = ? AND password_hash = ? AND token_version = ?", credential.ID, credential.PasswordHash, credential.TokenVersion).
		Select("PasswordHash", "TokenVersion").
		Updates(database.AdminCredential{PasswordHash: hash, TokenVersion: nextVersion})
	if result.Error != nil {
		return TokenPair{}, fmt.Errorf("save admin credential: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return TokenPair{}, ErrInvalidCredentials
	}
	return m.replacementSessionPair(ctx, nextVersion)
}

func (m *Manager) credential(ctx context.Context) (database.AdminCredential, error) {
	var credential database.AdminCredential
	if err := m.db.WithContext(ctx).First(&credential, 1).Error; err != nil {
		return database.AdminCredential{}, fmt.Errorf("read admin credential: %w", err)
	}
	return credential, nil
}

func (m *Manager) verifyVersion(ctx context.Context, version uint64) error {
	credential, err := m.credential(ctx)
	if err != nil {
		return err
	}
	if credential.TokenVersion != version {
		return ErrInvalidToken
	}
	return nil
}

func (m *Manager) newSessionPair(ctx context.Context, version uint64) (TokenPair, error) {
	sessionID, err := randomID()
	if err != nil {
		return TokenPair{}, err
	}
	pair, session, err := m.prepareSessionPair(version, sessionID)
	if err != nil {
		return TokenPair{}, err
	}
	if err := m.sessions.Create(ctx, session); err != nil {
		return TokenPair{}, err
	}
	return pair, nil
}

func (m *Manager) replacementSessionPair(ctx context.Context, version uint64) (TokenPair, error) {
	sessionID, err := randomID()
	if err != nil {
		return TokenPair{}, err
	}
	pair, session, err := m.prepareSessionPair(version, sessionID)
	if err != nil {
		return TokenPair{}, err
	}
	if err := m.sessions.ReplaceAll(ctx, session); err != nil {
		return TokenPair{}, err
	}
	return pair, nil
}

func (m *Manager) prepareSessionPair(version uint64, sessionID string) (TokenPair, refreshSession, error) {
	access, err := m.issue(tokenClaims{Kind: "access", Version: version}, AccessLifetime)
	if err != nil {
		return TokenPair{}, refreshSession{}, err
	}
	refresh, err := m.issue(tokenClaims{Kind: "refresh", Version: version, SessionID: sessionID}, RefreshLifetime)
	if err != nil {
		return TokenPair{}, refreshSession{}, err
	}
	pair := TokenPair{AccessToken: access, RefreshToken: refresh}
	session := refreshSession{
		id: sessionID, tokenHash: sha256.Sum256([]byte(refresh)), version: version,
		expiresAt: m.now().UTC().Add(RefreshLifetime),
	}
	return pair, session, nil
}

func (m *Manager) issue(claims tokenClaims, lifetime time.Duration) (string, error) {
	now := m.now().UTC()
	tokenID, err := randomID()
	if err != nil {
		return "", err
	}
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer: tokenIssuer, Subject: "admin", Audience: jwt.ClaimStrings{tokenAudience},
		ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)), IssuedAt: jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now), ID: tokenID,
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.key)
	if err != nil {
		return "", fmt.Errorf("sign %s token: %w", claims.Kind, err)
	}
	return value, nil
}

func (m *Manager) parse(value, kind string) (tokenClaims, error) {
	claims := tokenClaims{}
	token, err := jwt.ParseWithClaims(value, &claims, func(token *jwt.Token) (any, error) {
		return m.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(tokenAudience), jwt.WithSubject("admin"), jwt.WithExpirationRequired(), jwt.WithIssuedAt(),
		jwt.WithStrictDecoding(), jwt.WithTimeFunc(m.now))
	if err != nil || !token.Valid || claims.Kind != kind || kind == "refresh" && claims.SessionID == "" {
		return tokenClaims{}, ErrInvalidToken
	}
	return claims, nil
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
