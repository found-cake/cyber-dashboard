package integrationtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestAuthenticationBoundaryAndJWTCookies(t *testing.T) {
	currentTime := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{
		enableAuth: true, password: &password, nvdAPIKey: "test-nvd-key",
		now: func() time.Time { return currentTime },
	})

	for _, endpoint := range []string{"/api/dashboard", "/api/daily/2026-08-13", "/api/reports"} {
		public := performRequest(t, server, http.MethodGet, endpoint, nil)
		if public.Code != http.StatusOK {
			t.Fatalf("public GET %s status = %d", endpoint, public.Code)
		}
	}
	bootstrapResponse := performRequest(t, server, http.MethodGet, "/api/bootstrap", nil)
	var bootstrap api.Bootstrap
	if err := json.Unmarshal(bootstrapResponse.Body.Bytes(), &bootstrap); err != nil {
		t.Fatal(err)
	}
	if !bootstrap.Auth.Enabled || bootstrap.Auth.Authenticated || len(bootstrap.Sources) != 0 || len(bootstrap.LLMPresets) != 0 || bootstrap.Settings.LLMBaseURL != "" {
		t.Fatalf("unauthenticated bootstrap exposed administrator settings: %+v", bootstrap)
	}
	for _, endpoint := range []string{"/api/collect", "/api/cves/refresh", "/api/reports"} {
		response := performRequest(t, server, http.MethodPost, endpoint, map[string]string{})
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("POST %s status = %d, want 401", endpoint, response.Code)
		}
	}
	for _, request := range []struct {
		method string
		path   string
	}{{http.MethodPut, "/api/settings"}, {http.MethodGet, "/api/llm/presets"}} {
		response := performRequest(t, server, request.method, request.path, map[string]string{})
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", request.method, request.path, response.Code)
		}
	}

	wrong := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: "not-the-password"})
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", wrong.Code)
	}
	login := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: password})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 2 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || !cookies[1].HttpOnly || cookies[1].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login cookies = %#v", cookies)
	}

	authenticatedBootstrap := performRequestWithCookies(t, server, http.MethodGet, "/api/bootstrap", nil, cookies)
	if err := json.Unmarshal(authenticatedBootstrap.Body.Bytes(), &bootstrap); err != nil {
		t.Fatal(err)
	}
	if !bootstrap.Auth.Authenticated || len(bootstrap.Sources) == 0 || len(bootstrap.LLMPresets) == 0 || bootstrap.Settings.LLMBaseURL == "" {
		t.Fatalf("authenticated bootstrap is incomplete: %+v", bootstrap)
	}

	currentTime = currentTime.Add(31 * time.Minute)
	expired := performRequestWithCookies(t, server, http.MethodPost, "/api/cves/refresh", nil, cookies)
	if expired.Code != http.StatusUnauthorized {
		t.Fatalf("expired access status = %d", expired.Code)
	}
	refresh := performRequestWithCookies(t, server, http.MethodPost, "/api/auth/refresh", nil, cookies)
	if refresh.Code != http.StatusOK || len(refresh.Result().Cookies()) != 2 {
		t.Fatalf("refresh status = %d, cookies = %#v", refresh.Code, refresh.Result().Cookies())
	}
}

func TestLoginRateLimit(t *testing.T) {
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{enableAuth: true, password: &password})
	for attempt := 1; attempt <= 5; attempt++ {
		response := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: "wrong-password"})
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", attempt, response.Code)
		}
	}
	blocked := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: password})
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") != "60" {
		t.Fatalf("blocked login status/retry-after = %d/%q", blocked.Code, blocked.Header().Get("Retry-After"))
	}
}

func TestLoginRejectsOversizedJSONBody(t *testing.T) {
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{enableAuth: true, password: &password})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"password":"`+strings.Repeat("x", 8*1024)+`"}`))
	request.Host = "example.com"
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized login status = %d, want 413", response.Code)
	}
}

func TestLoginRejectsTrailingJSONValue(t *testing.T) {
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{enableAuth: true, password: &password})
	body, err := json.Marshal(api.LoginRequest{Password: password})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body)+` {}`))
	request.Host = "example.com"
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("login with trailing JSON status = %d, want 400", response.Code)
	}
}

func TestRefreshRotatesTokenAndRejectsReplay(t *testing.T) {
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{enableAuth: true, password: &password})
	login := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: password})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}
	originalCookies := login.Result().Cookies()

	rotated := performRequestWithCookies(t, server, http.MethodPost, "/api/auth/refresh", nil, originalCookies)
	if rotated.Code != http.StatusOK {
		t.Fatalf("first refresh status = %d", rotated.Code)
	}
	replayed := performRequestWithCookies(t, server, http.MethodPost, "/api/auth/refresh", nil, originalCookies)

	if replayed.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh status = %d, want 401", replayed.Code)
	}
}

func TestLogoutRevokesCopiedRefreshToken(t *testing.T) {
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{enableAuth: true, password: &password})
	login := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: password})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}
	copiedCookies := login.Result().Cookies()

	logout := performRequestWithCookies(t, server, http.MethodPost, "/api/auth/logout", nil, copiedCookies)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logout.Code)
	}
	replayed := performRequestWithCookies(t, server, http.MethodPost, "/api/auth/refresh", nil, copiedCookies)

	if replayed.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status = %d, want 401", replayed.Code)
	}
}

func TestConcurrentRefreshAllowsOneRotationWinner(t *testing.T) {
	password := ""
	server, _, _ := newTestServerWithConfig(t, testServerConfig{enableAuth: true, password: &password})
	login := performRequest(t, server, http.MethodPost, "/api/auth/login", api.LoginRequest{Password: password})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}
	originalCookies := login.Result().Cookies()
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() {
			request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
			request.Host = "example.com"
			for _, cookie := range originalCookies {
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			<-start
			server.ServeHTTP(response, request)
			results <- response
		}()
	}

	close(start)
	responses := []*httptest.ResponseRecorder{<-results, <-results}
	statuses := []int{responses[0].Code, responses[1].Code}
	sort.Ints(statuses)
	if statuses[0] != http.StatusOK || statuses[1] != http.StatusUnauthorized {
		t.Fatalf("concurrent refresh statuses = %v, want [200 401]", statuses)
	}
	var winner *httptest.ResponseRecorder
	for _, response := range responses {
		if response.Code == http.StatusOK {
			winner = response
		}
	}
	rotatedAgain := performRequestWithCookies(t, server, http.MethodPost, "/api/auth/refresh", nil, winner.Result().Cookies())
	if rotatedAgain.Code != http.StatusOK {
		t.Fatalf("winning refresh token status = %d", rotatedAgain.Code)
	}
}

func performRequest(t *testing.T, server http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	return performRequestWithCookies(t, server, method, path, body, nil)
}

func performRequestWithCookies(t *testing.T, server http.Handler, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, &encoded)
	request.Host = "example.com"
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}
