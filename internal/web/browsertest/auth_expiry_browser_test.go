//go:build browser

package browsertest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

type authExpiryCounts struct {
	bootstrap, dashboard, login, refresh, settings atomic.Int32
}

func TestExpiredSessionResumesProtectedRequestAfterLogin(t *testing.T) {
	viewports := []struct{ width, height int64 }{{375, 812}, {768, 900}, {1280, 900}}
	records := make([]authScreenshot, 0, len(viewports))

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dpx", viewport.width), func(t *testing.T) {
			// Given an authenticated settings view whose next protected request expires.
			server, counts := newAuthExpiryBrowserServer(t)
			root, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			t.Cleanup(cancel)
			browser, browserCancel := chromedp.NewContext(root)
			t.Cleanup(browserCancel)
			var exceptions browserRuntimeExceptions
			chromedp.ListenTarget(browser, exceptions.listen)
			evidence := authEvidence{os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR"), viewport.width, viewport.height, &records}

			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`#main-content .empty-state`),
			); err != nil {
				t.Fatalf("load authenticated dashboard: %v", err)
			}
			var authChrome struct {
				SettingsHidden bool `json:"settingsHidden"`
				LogoutHidden   bool `json:"logoutHidden"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				settingsHidden: document.querySelector('#settings-action').hidden,
				logoutHidden: document.querySelector('#auth-action').hidden
			})`, &authChrome)); err != nil {
				t.Fatalf("inspect authenticated chrome: %v", err)
			}
			if authChrome.SettingsHidden || authChrome.LogoutHidden {
				t.Fatalf("auth chrome = %+v, refreshes = %d, bootstraps = %d; want authenticated controls", authChrome, counts.refresh.Load(), counts.bootstrap.Load())
			}
			openSettingsPage(t, browser, viewport.width)

			// When saving encounters a 401 and refresh also fails.
			if err := chromedp.Run(browser,
				chromedp.SetValue(`#llm-timeout`, "75"),
				chromedp.Click(`#save-settings`),
				chromedp.WaitVisible(`#login-form`),
				chromedp.Poll(`document.querySelector('#page-title').textContent === 'Settings' && document.querySelector('#llm-timeout').value === '75'`, nil),
			); err != nil {
				t.Fatalf("wait for expired-session login: %v; runtime exceptions: %s", err, exceptions.String())
			}
			evidence.capture(t, browser, "expired-modal")
			if got, want := ([5]int32{counts.bootstrap.Load(), counts.dashboard.Load(), counts.login.Load(), counts.refresh.Load(), counts.settings.Load()}), ([5]int32{1, 1, 0, 2, 1}); got != want {
				t.Fatalf("expired-session request counts = %+v, want %+v", got, want)
			}

			// Then login resumes that same save once and preserves the settings view.
			if err := chromedp.Run(browser,
				chromedp.SetValue(`#login-password`, "correct-password"),
				chromedp.Click(`#submit-login`),
				chromedp.Poll(`!document.querySelector('#auth-action').hidden && !document.querySelector('#login-form') && document.querySelector('#page-title').textContent === 'Settings' && document.querySelector('#llm-timeout').value === '75' && document.querySelector('#toast-region .toast')`, nil, chromedp.WithPollingTimeout(3*time.Second)),
			); err != nil {
				t.Fatalf("resume settings save after login: %v; runtime exceptions: %s", err, exceptions.String())
			}
			if got, want := ([5]int32{counts.bootstrap.Load(), counts.dashboard.Load(), counts.login.Load(), counts.refresh.Load(), counts.settings.Load()}), ([5]int32{2, 1, 1, 2, 2}); got != want {
				t.Fatalf("resumed request counts = %+v, want %+v", got, want)
			}
			if got := exceptions.String(); got != "" {
				t.Fatalf("runtime exceptions after session recovery:\n%s", got)
			}
		})
	}

	writeAuthExpiryManifest(t, os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR"), records)
}

func newAuthExpiryBrowserServer(t *testing.T) (*httptest.Server, *authExpiryCounts) {
	t.Helper()
	counts := &authExpiryCounts{}
	var authenticated atomic.Bool
	authenticated.Store(true)
	settings := api.SettingsResponse{
		Language: "en", Accent: "#4f6ef7", LLMBaseURL: "https://api.openai.com/v1",
		LLMModel: "gpt-4o-mini", LLMTimeout: 60, TimezoneOffsetMinutes: 540,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		counts.bootstrap.Add(1)
		writeJSON(t, writer, api.Bootstrap{
			Auth:    api.AuthState{Enabled: true, Authenticated: authenticated.Load()},
			Sources: []api.Source{}, Reports: []api.Report{}, Settings: settings,
			LLMPresets: []api.LLMPresetResponse{}, CollectedDays: []string{},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		counts.dashboard.Add(1)
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("POST /api/auth/refresh", func(writer http.ResponseWriter, _ *http.Request) {
		if counts.refresh.Add(1) == 1 {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		authenticated.Store(false)
		writeJSONStatus(t, writer, http.StatusUnauthorized, api.ErrorResponse{Message: "session expired"})
	})
	mux.HandleFunc("POST /api/auth/login", func(writer http.ResponseWriter, request *http.Request) {
		counts.login.Add(1)
		var login api.LoginRequest
		if err := json.NewDecoder(request.Body).Decode(&login); err != nil || login.Password != "correct-password" {
			writeJSONStatus(t, writer, http.StatusUnauthorized, api.ErrorResponse{Message: "wrong password"})
			return
		}
		authenticated.Store(true)
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /api/settings", func(writer http.ResponseWriter, request *http.Request) {
		if counts.settings.Add(1) == 1 {
			authenticated.Store(false)
			writeJSONStatus(t, writer, http.StatusUnauthorized, api.ErrorResponse{Message: "session expired"})
			return
		}
		var saved api.Settings
		if err := json.NewDecoder(request.Body).Decode(&saved); err != nil {
			writeJSONStatus(t, writer, http.StatusBadRequest, api.ErrorResponse{Message: "invalid settings"})
			return
		}
		settings.LLMTimeout = saved.LLMTimeout
		writeJSON(t, writer, settings)
	})
	mux.Handle("/", http.FileServerFS(os.DirFS("../../../static")))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, counts
}

func writeAuthExpiryManifest(t *testing.T, directory string, records []authScreenshot) {
	t.Helper()
	if directory == "" {
		return
	}
	document, err := json.MarshalIndent(struct {
		Scenario    string           `json:"scenario"`
		Screenshots []authScreenshot `json:"screenshots"`
	}{"expired-session-resume", records}, "", "  ")
	if err != nil {
		t.Fatalf("encode auth expiry manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "auth-expiry-manifest.json"), document, 0o644); err != nil {
		t.Fatalf("write auth expiry manifest: %v", err)
	}
}
