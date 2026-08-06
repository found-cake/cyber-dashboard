//go:build browser

package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestSettingsKeepsStoredAPIKeysOutOfInputs_whenPageLoadsAndPresetChanges(t *testing.T) {
	// Given a browser bootstrap response that exposes only credential status flags.
	savedRequests := make(chan api.Settings, 3)
	server := newSettingsSecurityBrowserServer(t, savedRequests)
	viewports := []struct {
		width  int64
		height int64
	}{
		{width: 375, height: 812},
		{width: 768, height: 900},
		{width: 1280, height: 900},
	}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dpx", viewport.width), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			t.Cleanup(cancel)
			browser, browserCancel := chromedp.NewContext(ctx)
			t.Cleanup(browserCancel)

			// When the settings page is opened and a configured preset is selected.
			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`#main-content .empty-state`),
			); err != nil {
				t.Fatalf("initialize dashboard: %v", err)
			}
			openSettingsPage(t, browser, viewport.width)
			captureSettingsEvidence(t, browser, server.URL, viewport.width, viewport.height)

			var initial struct {
				LLMValue       string `json:"llmValue"`
				NVDValue       string `json:"nvdValue"`
				LLMPlaceholder string `json:"llmPlaceholder"`
				NVDPlaceholder string `json:"nvdPlaceholder"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				llmValue: document.querySelector("#llm-api-key").value,
				nvdValue: document.querySelector("#nvd-api-key").value,
				llmPlaceholder: document.querySelector("#llm-api-key").placeholder,
				nvdPlaceholder: document.querySelector("#nvd-api-key").placeholder
			})`, &initial)); err != nil {
				t.Fatalf("inspect settings: %v", err)
			}

			// Then both inputs are empty while their non-secret saved state remains visible.
			if initial.LLMValue != "" || initial.NVDValue != "" {
				t.Fatalf("key input values = %q / %q, want empty", initial.LLMValue, initial.NVDValue)
			}
			if initial.LLMPlaceholder == "" || initial.NVDPlaceholder == "" {
				t.Fatalf("key placeholders = %q / %q, want saved-state guidance", initial.LLMPlaceholder, initial.NVDPlaceholder)
			}

			if err := chromedp.Run(browser,
				chromedp.SetValue(`#llm-api-key`, "unsaved-preset-secret"),
				chromedp.Evaluate(`document.querySelector('[data-preset-id="2"]').click()`, nil),
				chromedp.Poll(`document.querySelector("#llm-api-key").value === ""`, nil),
				chromedp.SetValue(`#llm-timeout`, "75"),
				chromedp.Click(`#save-settings`),
				chromedp.WaitVisible(`.toast`),
				chromedp.WaitVisible(`#llm-api-key`),
			); err != nil {
				t.Fatalf("save selected preset: %v", err)
			}
			var savedValue string
			if err := chromedp.Run(browser, chromedp.Value(`#llm-api-key`, &savedValue)); err != nil {
				t.Fatalf("inspect saved key input: %v", err)
			}
			if savedValue != "" {
				t.Fatalf("saved key input = %q, want empty", savedValue)
			}
			select {
			case request := <-savedRequests:
				if request.LLMAPIKey != "" || request.NVDAPIKey != "" {
					t.Fatalf("browser submitted keys = %q / %q, want blank preservation markers", request.LLMAPIKey, request.NVDAPIKey)
				}
			default:
				t.Fatal("settings save request was not observed")
			}

			// When a preset action rerenders a draft containing newly typed credentials.
			if err := chromedp.Run(browser,
				chromedp.SetValue(`#llm-api-key`, "unsaved-llm-secret"),
				chromedp.SetValue(`#nvd-api-key`, "unsaved-nvd-secret"),
				chromedp.Evaluate(`document.querySelector('[data-remove-preset-id="2"]').click()`, nil),
				chromedp.WaitNotPresent(`[data-preset-id="2"]`),
			); err != nil {
				t.Fatalf("remove preset with credential draft: %v", err)
			}
			var rerendered struct {
				LLM string `json:"llm"`
				NVD string `json:"nvd"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				llm: document.querySelector("#llm-api-key").value,
				nvd: document.querySelector("#nvd-api-key").value
			})`, &rerendered)); err != nil {
				t.Fatalf("inspect rerendered credential inputs: %v", err)
			}

			// Then the rerender clears both plaintext drafts instead of restoring them.
			if rerendered.LLM != "" || rerendered.NVD != "" {
				t.Fatalf("rerendered key input values = %q / %q, want empty", rerendered.LLM, rerendered.NVD)
			}
		})
	}
}

func TestLanguageSelectionLivesInSettingsAndAppliesAfterSave(t *testing.T) {
	savedRequests := make(chan api.Settings, 3)
	server := newSettingsSecurityBrowserServer(t, savedRequests)
	viewports := []struct {
		width  int64
		height int64
	}{{375, 812}, {768, 900}, {1280, 900}}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dpx", viewport.width), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			t.Cleanup(cancel)
			browser, browserCancel := chromedp.NewContext(ctx)
			t.Cleanup(browserCancel)

			var headerControls int
			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`#main-content .empty-state`),
				chromedp.Evaluate(`document.querySelectorAll('.topbar [data-lang], .topbar select').length`, &headerControls),
			); err != nil {
				t.Fatalf("initialize dashboard: %v", err)
			}
			if headerControls != 0 {
				t.Fatalf("header language controls = %d, want none", headerControls)
			}

			openSettingsPage(t, browser, viewport.width)
			if directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR"); directory != "" {
				warmSettingsCapture(t, browser, viewport.width)
				screenshot := captureSettingsTarget(t, browser, "#setting-language", viewport.width, viewport.height)
				writeSettingsScreenshot(t, directory, "language", viewport.width, screenshot)
			}
			var options int
			if err := chromedp.Run(browser,
				chromedp.WaitVisible(`#setting-language`),
				chromedp.Evaluate(`document.querySelector('#setting-language').options.length`, &options),
				chromedp.Evaluate(`(() => { const select = document.querySelector('#setting-language'); select.value = 'en'; select.dispatchEvent(new Event('change', { bubbles: true })); })()`, nil),
				chromedp.WaitVisible(`#settings-save-bar`),
				chromedp.Poll(`document.documentElement.lang === 'ko'`, nil),
				chromedp.Click(`#save-settings`),
				chromedp.Poll(`document.documentElement.lang === 'en' && document.querySelector('#page-title').textContent === 'Settings'`, nil),
			); err != nil {
				t.Fatalf("select and save language: %v", err)
			}
			if options != 2 {
				t.Fatalf("language options = %d, want 2", options)
			}

			select {
			case saved := <-savedRequests:
				if saved.Language != "en" {
					t.Fatalf("saved language = %q, want en", saved.Language)
				}
			default:
				t.Fatal("settings save request was not received")
			}
		})
	}
}

func newSettingsSecurityBrowserServer(t *testing.T, savedRequests chan<- api.Settings) *httptest.Server {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Sources: []api.Source{}, Reports: []api.Report{}, CollectedDays: []string{},
			Settings: api.SettingsResponse{
				Language: "ko", Theme: "dark", Accent: "#4f6ef7",
				LLMBaseURL: "https://api.openai.com/v1", LLMModel: "gpt-4o-mini",
				LLMAPIKeyConfigured: true, LLMTimeout: 60, NVDAPIKeyConfigured: true,
				TimezoneOffsetMinutes: 540,
			},
			LLMPresets: []api.LLMPresetResponse{
				{ID: 1, Label: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", APIKeyConfigured: true, Builtin: true},
				{ID: 2, Label: "localhost:11434", BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKeyConfigured: true},
			},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("PUT /api/settings", func(writer http.ResponseWriter, request *http.Request) {
		var value api.Settings
		if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
			t.Errorf("decode settings request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		savedRequests <- value
		writeJSON(t, writer, api.SettingsResponse{
			Language: value.Language, Theme: value.Theme, Accent: value.Accent,
			LLMBaseURL: value.LLMBaseURL, LLMModel: value.LLMModel,
			LLMAPIKeyConfigured: true, LLMTimeout: value.LLMTimeout, NVDAPIKeyConfigured: true,
			TimezoneOffsetMinutes: value.TimezoneOffsetMinutes,
		})
	})
	mux.HandleFunc("DELETE /api/llm/presets/2", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
