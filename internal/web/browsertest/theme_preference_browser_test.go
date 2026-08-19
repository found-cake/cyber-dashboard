//go:build browser

package browsertest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestThemeFollowsSystemPreference_whenNothingIsStored(t *testing.T) {
	// Given a bootstrap response that carries no theme, because the server no longer owns one.
	server := newThemePreferenceBrowserServer(t, make(chan map[string]json.RawMessage, 1))
	tests := []struct {
		scheme string
		want   string
	}{
		{scheme: "light", want: "light"},
		{scheme: "dark", want: "dark"},
	}

	for _, test := range tests {
		t.Run(test.scheme, func(t *testing.T) {
			browser := newBrowserContext(t, 20*time.Second)

			// When a first-time visitor loads the dashboard under that system preference.
			var applied, stored string
			if err := chromedp.Run(browser,
				emulateColorScheme(test.scheme),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`#main-content .empty-state`),
				chromedp.Evaluate(`document.documentElement.dataset.theme`, &applied),
				chromedp.Evaluate(`String(localStorage.getItem("cyber-theme"))`, &stored),
			); err != nil {
				t.Fatalf("load dashboard: %v", err)
			}

			// Then the system preference is applied without hardening into a stored choice.
			if applied != test.want {
				t.Errorf("applied theme = %q, want %q", applied, test.want)
			}
			if stored != "null" {
				t.Errorf("stored theme = %q, want nothing stored", stored)
			}
		})
	}
}

func TestThemeTogglePinsChoiceLocally_whenSystemPrefersTheOtherScheme(t *testing.T) {
	// Given a dashboard loaded under a dark system preference.
	savedRequests := make(chan map[string]json.RawMessage, 1)
	server := newThemePreferenceBrowserServer(t, savedRequests)
	browser := newBrowserContext(t, 30*time.Second)

	// When the theme is toggled to light and the page is reloaded.
	var toggled, saveBarHidden, reloaded, stored string
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		emulateColorScheme("dark"),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#main-content .empty-state`),
		chromedp.Click(`#theme-toggle`),
		chromedp.Evaluate(`document.documentElement.dataset.theme`, &toggled),
	); err != nil {
		t.Fatalf("toggle theme: %v", err)
	}
	openSettingsPage(t, browser, 1280)
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`String(document.querySelector("#settings-save-bar").hidden)`, &saveBarHidden),
		chromedp.Reload(),
		chromedp.WaitVisible(`#main-content .empty-state, #setting-language`),
		chromedp.Evaluate(`document.documentElement.dataset.theme`, &reloaded),
		chromedp.Evaluate(`String(localStorage.getItem("cyber-theme"))`, &stored),
	); err != nil {
		t.Fatalf("reload after toggle: %v", err)
	}

	// Then the explicit choice outlives the reload and keeps beating the system preference.
	if toggled != "light" {
		t.Errorf("toggled theme = %q, want light", toggled)
	}
	if reloaded != "light" || stored != "light" {
		t.Errorf("after reload theme = %q stored = %q, want light and light", reloaded, stored)
	}
	// And the toggle never marks settings dirty, because the theme left the settings contract.
	if saveBarHidden != "true" {
		t.Errorf("settings save bar hidden = %q, want true", saveBarHidden)
	}
}

func TestSettingsSaveOmitsTheme_whenLanguageChanges(t *testing.T) {
	// Given the settings page with a pending language change.
	savedRequests := make(chan map[string]json.RawMessage, 1)
	server := newThemePreferenceBrowserServer(t, savedRequests)
	browser := newBrowserContext(t, 30*time.Second)

	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#main-content .empty-state`),
	); err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	openSettingsPage(t, browser, 1280)

	// When the language is changed and saved.
	if err := chromedp.Run(browser,
		chromedp.SetValue(`#setting-language`, "ko", chromedp.ByQuery),
		chromedp.WaitVisible(`#save-settings`),
		chromedp.Click(`#save-settings`),
	); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	// Then the request carries the language and no theme field at all.
	select {
	case saved := <-savedRequests:
		var language string
		if err := json.Unmarshal(saved["language"], &language); err != nil {
			t.Fatalf("decode saved language: %v", err)
		}
		if language != "ko" {
			t.Errorf("saved language = %q, want ko", language)
		}
		if _, exists := saved["theme"]; exists {
			t.Error("settings request contains browser-local theme")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("settings save never reached the server")
	}
}

func emulateColorScheme(scheme string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		return emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
			{Name: "prefers-color-scheme", Value: scheme},
		}).Do(ctx)
	}
}

func newThemePreferenceBrowserServer(t *testing.T, savedRequests chan<- map[string]json.RawMessage) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Sources: []api.Source{}, Reports: []api.ReportSummary{}, CollectedDays: []string{},
			Settings: api.SettingsResponse{
				Language: "en", Accent: "#4f6ef7", LLMBaseURL: "https://api.openai.com/v1",
				LLMModel: "gpt-4o-mini", LLMTimeout: 60, TimezoneOffsetMinutes: 540,
			},
			LLMPresets: []api.LLMPresetResponse{},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("PUT /api/settings", func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read settings request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			t.Errorf("decode settings request fields: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var value api.Settings
		if err := json.Unmarshal(body, &value); err != nil {
			t.Errorf("decode settings request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		savedRequests <- fields
		writeJSON(t, writer, api.SettingsResponse{
			Language: value.Language, Accent: value.Accent, LLMBaseURL: value.LLMBaseURL,
			LLMModel: value.LLMModel, LLMTimeout: value.LLMTimeout,
			TimezoneOffsetMinutes: value.TimezoneOffsetMinutes,
		})
	})
	mux.Handle("/", http.FileServerFS(os.DirFS("../../../static")))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
