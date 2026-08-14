package integrationtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/found-cake/cyber-dashboard/internal/auth"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/web"
)

func TestAuthenticationReturnsInternalServerError_whenCredentialDatabaseUnavailable(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		cookieName string
		token      func(auth.TokenPair) string
	}{
		{
			name:       "access token verification",
			path:       "/api/reports",
			cookieName: "cyber_dashboard_access",
			token:      func(pair auth.TokenPair) string { return pair.AccessToken },
		},
		{
			name:       "refresh token verification",
			path:       "/api/auth/refresh",
			cookieName: "cyber_dashboard_refresh",
			token:      func(pair auth.TokenPair) string { return pair.RefreshToken },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given a valid session whose credential database becomes unavailable.
			server, pair := newServerWithUnavailableCredentialDatabase(t)
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			request.Host = "example.com"
			request.AddCookie(&http.Cookie{Name: test.cookieName, Value: test.token(pair)})
			response := httptest.NewRecorder()

			// When the session is verified through the HTTP boundary.
			server.ServeHTTP(response, request)

			// Then the storage failure remains a server error and no session cookie is cleared.
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", response.Code)
			}
			if cookies := response.Result().Cookies(); len(cookies) != 0 {
				t.Fatalf("response cleared cookies on database failure: %#v", cookies)
			}
		})
	}
}

func newServerWithUnavailableCredentialDatabase(t *testing.T) (*web.Server, auth.TokenPair) {
	t.Helper()
	ctx := context.Background()
	directory := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(directory, "dashboard.db"))
	if err != nil {
		t.Fatalf("open credential database: %v", err)
	}
	databaseClosed := false
	t.Cleanup(func() {
		if !databaseClosed {
			if err := database.Close(db); err != nil {
				t.Errorf("close credential database: %v", err)
			}
		}
	})
	sessions, err := auth.OpenSessionStore(ctx, filepath.Join(directory, "dashboard.sessions.db"), nil)
	if err != nil {
		t.Fatalf("open refresh sessions: %v", err)
	}
	t.Cleanup(func() {
		if err := sessions.Close(); err != nil {
			t.Errorf("close refresh sessions: %v", err)
		}
	})
	manager, err := auth.NewManager(db, sessions, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("open authentication: %v", err)
	}
	password, _, err := manager.EnsurePassword(ctx)
	if err != nil {
		t.Fatalf("initialize authentication: %v", err)
	}
	pair, err := manager.Login(ctx, password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := database.Close(db); err != nil {
		t.Fatalf("close credential database: %v", err)
	}
	databaseClosed = true
	server := web.NewServer(web.Dependencies{
		Assets:       fstest.MapFS{"index.html": {Data: []byte("ok")}},
		TrustedHosts: []string{"example.com"},
		Auth:         manager,
	})
	return server, pair
}
