//go:build browser

package browsertest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

type authRequestCounts struct {
	bootstrap, dashboard, login, logout, refresh atomic.Int32
}

type browserRuntimeExceptions struct {
	mu     sync.Mutex
	values []string
}

func (exceptions *browserRuntimeExceptions) listen(event any) {
	var message string
	switch event := event.(type) {
	case *cdpruntime.EventExceptionThrown:
		if event.ExceptionDetails != nil {
			message = event.ExceptionDetails.Error()
		}
	case *cdpruntime.EventConsoleAPICalled:
		if event.Type != cdpruntime.APITypeError && event.Type != cdpruntime.APITypeWarning {
			return
		}
		for _, argument := range event.Args {
			message += " " + argument.Description
		}
		message = strings.TrimSpace(message)
	}
	if !strings.Contains(message, "Error") && !strings.Contains(message, "exception") {
		return
	}
	exceptions.mu.Lock()
	defer exceptions.mu.Unlock()
	exceptions.values = append(exceptions.values, message)
}

func (exceptions *browserRuntimeExceptions) String() string {
	exceptions.mu.Lock()
	defer exceptions.mu.Unlock()
	return strings.Join(exceptions.values, "\n")
}

type authScreenshot struct {
	Width int    `json:"width"`
	State string `json:"state"`
	File  string `json:"file"`
}

type authEvidence struct {
	directory     string
	width, height int64
	records       *[]authScreenshot
}

func (evidence authEvidence) capture(t *testing.T, browser context.Context, state string) {
	t.Helper()
	if evidence.directory == "" {
		return
	}
	if err := os.MkdirAll(evidence.directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	var screenshot []byte
	if err := chromedp.Run(browser,
		chromedp.Poll(`(() => { const modal = document.querySelector('#login-form'); if (!modal) return true; const style = getComputedStyle(modal); return style.opacity === '1' && style.transform === 'none'; })()`, nil),
		chromedp.Evaluate(`new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`, nil),
		chromedp.CaptureScreenshot(&screenshot),
	); err != nil {
		t.Fatalf("capture %s at %dpx: %v", state, evidence.width, err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(screenshot))
	if err != nil {
		t.Fatalf("decode %s screenshot at %dpx: %v", state, evidence.width, err)
	}
	if int64(config.Width) != evidence.width || int64(config.Height) != evidence.height {
		t.Fatalf("%s screenshot dimensions = %dx%d, want %dx%d", state, config.Width, config.Height, evidence.width, evidence.height)
	}
	name := fmt.Sprintf("auth-%d-%s.png", evidence.width, state)
	if err := os.WriteFile(filepath.Join(evidence.directory, name), screenshot, 0o644); err != nil {
		t.Fatalf("write %s screenshot: %v", state, err)
	}
	*evidence.records = append(*evidence.records, authScreenshot{Width: int(evidence.width), State: state, File: name})
}

func TestAuthRoutingRestoresCurrentView_whenLoginAndLogoutComplete(t *testing.T) {
	viewports := []struct{ width, height int64 }{{375, 812}, {768, 900}, {1280, 900}}
	records := make([]authScreenshot, 0, len(viewports)*4)

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dpx", viewport.width), func(t *testing.T) {
			// Given an unauthenticated dashboard backed by a stateful auth fixture.
			server, counts := newAuthBrowserServer(t)
			root, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)
			browser, browserCancel := chromedp.NewContext(root)
			t.Cleanup(browserCancel)
			var exceptions browserRuntimeExceptions
			chromedp.ListenTarget(browser, exceptions.listen)
			evidence := authEvidence{os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR"), viewport.width, viewport.height, &records}

			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.Poll(`document.querySelector('.stat-card strong')?.textContent === '1'`, nil),
			); err != nil {
				t.Fatalf("load logged-out dashboard: %v; runtime exceptions: %s", err, exceptions.String())
			}
			openAuthLogin(t, browser, viewport.width)
			evidence.capture(t, browser, "login-modal")

			// When a wrong password is submitted.
			if err := chromedp.Run(browser,
				chromedp.SetValue(`#login-password`, "wrong-password"),
				chromedp.Click(`#submit-login`),
				chromedp.WaitVisible(`#login-form .field-error .field-message`),
			); err != nil {
				t.Fatalf("submit wrong password: %v", err)
			}
			var wrongPassword struct {
				Modal   bool   `json:"modal"`
				Message string `json:"message"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				modal: document.querySelector('#login-form') !== null,
				message: document.querySelector('#login-form .field-message').textContent
			})`, &wrongPassword)); err != nil {
				t.Fatalf("inspect wrong-password modal: %v", err)
			}
			if !wrongPassword.Modal || wrongPassword.Message == "" {
				t.Fatalf("wrong-password state = %+v, want open modal with inline error", wrongPassword)
			}
			evidence.capture(t, browser, "wrong-password")
			if got, want := ([5]int32{counts.bootstrap.Load(), counts.dashboard.Load(), counts.login.Load(), counts.logout.Load(), counts.refresh.Load()}), ([5]int32{1, 1, 1, 0, 1}); got != want {
				t.Fatalf("wrong-password request counts = %+v, want %+v", got, want)
			}

			// When the correct password is submitted, then the current dashboard is rerendered.
			if err := chromedp.Run(browser,
				chromedp.SetValue(`#login-password`, "correct-password"),
				chromedp.Click(`#submit-login`),
				chromedp.Poll(`!document.querySelector('#auth-action').hidden && document.querySelector('.stat-card strong')?.textContent === '2'`, nil, chromedp.WithPollingTimeout(3*time.Second)),
			); err != nil {
				t.Fatalf("wait for authenticated dashboard: %v; runtime exceptions: %s", err, exceptions.String())
			}
			evidence.capture(t, browser, "authenticated")

			// When logout completes, then the logged-out dashboard is rerendered.
			if err := chromedp.Run(browser,
				chromedp.Click(`#auth-action`),
				chromedp.Poll(`document.querySelector('#auth-action').hidden && document.querySelector('.stat-card strong')?.textContent === '3'`, nil, chromedp.WithPollingTimeout(3*time.Second)),
			); err != nil {
				t.Fatalf("wait for logged-out dashboard: %v; runtime exceptions: %s", err, exceptions.String())
			}
			evidence.capture(t, browser, "logged-out")
			if got, want := ([5]int32{counts.bootstrap.Load(), counts.dashboard.Load(), counts.login.Load(), counts.logout.Load(), counts.refresh.Load()}), ([5]int32{3, 3, 2, 1, 1}); got != want {
				t.Fatalf("completed auth request counts = %+v, want %+v", got, want)
			}
			if got := exceptions.String(); got != "" {
				t.Fatalf("runtime exceptions after auth flow:\n%s", got)
			}
		})
	}

	writeAuthManifest(t, os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR"), records)
}

func openAuthLogin(t *testing.T, browser context.Context, width int64) {
	t.Helper()
	if width <= 900 {
		if err := chromedp.Run(browser,
			chromedp.Click(`#menu-button`),
			chromedp.Poll(`(() => {
				const element = document.querySelector('#settings-action');
				const rect = element.getBoundingClientRect();
				const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
				return rect.left >= 0 && rect.right <= innerWidth && (hit === element || element.contains(hit));
			})()`, nil),
		); err != nil {
			t.Fatalf("open compact navigation: %v", err)
		}
	}
	if err := chromedp.Run(browser, chromedp.Click(`#settings-action`), chromedp.WaitVisible(`#login-form`)); err != nil {
		t.Fatalf("open login modal: %v", err)
	}
	if width <= 900 {
		if err := chromedp.Run(browser, chromedp.Poll(`!document.querySelector('.app-shell').classList.contains('is-drawer-open') && document.querySelector('#sidebar').getBoundingClientRect().right <= 0`, nil)); err != nil {
			t.Fatalf("wait for compact navigation to close: %v", err)
		}
	}
}

func newAuthBrowserServer(t *testing.T) (*httptest.Server, *authRequestCounts) {
	t.Helper()
	counts := &authRequestCounts{}
	var authenticated atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		counts.bootstrap.Add(1)
		writeJSON(t, writer, api.Bootstrap{
			Auth:     api.AuthState{Enabled: true, Authenticated: authenticated.Load()},
			Settings: api.SettingsResponse{Language: "en", TimezoneOffsetMinutes: 540},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		call := counts.dashboard.Add(1)
		writeJSON(t, writer, api.Dashboard{Total: int(call), CVECount: int(call)})
	})
	mux.HandleFunc("POST /api/auth/refresh", func(writer http.ResponseWriter, _ *http.Request) {
		counts.refresh.Add(1)
		writeJSONStatus(t, writer, http.StatusUnauthorized, api.ErrorResponse{Message: "session unavailable"})
	})
	mux.HandleFunc("POST /api/auth/login", func(writer http.ResponseWriter, request *http.Request) {
		counts.login.Add(1)
		var login api.LoginRequest
		if err := json.NewDecoder(request.Body).Decode(&login); err != nil {
			writeJSONStatus(t, writer, http.StatusBadRequest, api.ErrorResponse{Message: "invalid request"})
			return
		}
		if login.Password != "correct-password" {
			writeJSONStatus(t, writer, http.StatusUnauthorized, api.ErrorResponse{Message: "wrong password"})
			return
		}
		authenticated.Store(true)
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/auth/logout", func(writer http.ResponseWriter, _ *http.Request) {
		counts.logout.Add(1)
		authenticated.Store(false)
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", http.FileServerFS(os.DirFS("../../../static")))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, counts
}

func writeAuthManifest(t *testing.T, directory string, records []authScreenshot) {
	t.Helper()
	if directory == "" {
		return
	}
	document, err := json.MarshalIndent(struct {
		Scenario    string           `json:"scenario"`
		Screenshots []authScreenshot `json:"screenshots"`
	}{"login-wrong-password-login-logout", records}, "", "  ")
	if err != nil {
		t.Fatalf("encode auth screenshot manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "auth-routing-manifest.json"), document, 0o644); err != nil {
		t.Fatalf("write auth screenshot manifest: %v", err)
	}
}
