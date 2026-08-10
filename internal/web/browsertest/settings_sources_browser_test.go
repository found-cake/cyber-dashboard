//go:build browser

package browsertest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestSourceSettingsWaitForSaveAndSupportRevert(t *testing.T) {
	viewports := []struct {
		width  int64
		height int64
	}{{375, 812}, {768, 900}, {1280, 900}}
	languages := []string{"en", "ko"}
	themes := []string{"light", "dark"}

	for _, language := range languages {
		for _, theme := range themes {
			for _, viewport := range viewports {
				t.Run(fmt.Sprintf("%s-%s-%dpx", language, theme, viewport.width), func(t *testing.T) {
					// Given BleepingComputer is enabled in persisted settings.
					savedRequests := make(chan api.SaveSettingsRequest, 1)
					server := newSourceSettingsBrowserServer(t, savedRequests, language)
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					t.Cleanup(cancel)
					browser, browserCancel := chromedp.NewContext(ctx)
					t.Cleanup(browserCancel)
					if err := chromedp.Run(browser,
						chromedp.EmulateViewport(viewport.width, viewport.height),
						chromedp.Navigate(server.URL),
						chromedp.WaitVisible(`#main-content .empty-state`),
					); err != nil {
						t.Fatalf("initialize dashboard: %v", err)
					}
					openSettingsPage(t, browser, viewport.width)
					if err := chromedp.Run(browser, chromedp.Evaluate(fmt.Sprintf(`localStorage.setItem("cyber-theme", %q); document.documentElement.dataset.theme = %q`, theme, theme), nil)); err != nil {
						t.Fatalf("apply %s theme: %v", theme, err)
					}

					// When the source is toggled without saving.
					if err := chromedp.Run(browser,
						chromedp.Click(`[data-source-id="2"]`),
						chromedp.WaitVisible(`#settings-save-bar`),
						chromedp.Poll(`document.querySelector('[data-source-id="2"]').getAttribute('aria-checked') === 'false'`, nil),
					); err != nil {
						t.Fatalf("draft source change: %v", err)
					}
					captureSourceSettingsScreenshot(t, browser, sourceSettingsCapture{
						width: viewport.width, language: language, theme: theme,
					})
					select {
					case <-savedRequests:
						t.Fatal("source was persisted before Save was activated")
					default:
					}

					// Then Revert restores the persisted source state and hides the action bar.
					if err := chromedp.Run(browser,
						chromedp.Click(`#revert-settings`),
						chromedp.Poll(`document.querySelector('#settings-save-bar').hidden && document.querySelector('[data-source-id="2"]').getAttribute('aria-checked') === 'true'`, nil),
					); err != nil {
						t.Fatalf("revert source draft: %v", err)
					}

					// When the source is toggled again and Save is activated.
					if err := chromedp.Run(browser,
						chromedp.Click(`[data-source-id="2"]`),
						chromedp.WaitVisible(`#settings-save-bar`),
						chromedp.Click(`#save-settings`),
						chromedp.WaitVisible(`#toast-region .toast`),
						chromedp.Poll(`document.querySelector('#settings-save-bar').hidden`, nil),
					); err != nil {
						t.Fatalf("save source draft: %v", err)
					}
					select {
					case request := <-savedRequests:
						if len(request.Sources) != 1 || request.Sources[0].ID != 2 || request.Sources[0].Enabled {
							t.Fatalf("saved sources = %+v, want BleepingComputer disabled", request.Sources)
						}
					default:
						t.Fatal("settings save request was not received")
					}

					// Then a reload shows the saved source state without an unsaved action bar.
					if err := chromedp.Run(browser,
						chromedp.Reload(),
						chromedp.WaitVisible(`#main-content .empty-state`),
					); err != nil {
						t.Fatalf("reload dashboard: %v", err)
					}
					openSettingsPage(t, browser, viewport.width)
					if err := chromedp.Run(browser, chromedp.Poll(`document.querySelector('#settings-save-bar').hidden && document.querySelector('[data-source-id="2"]').getAttribute('aria-checked') === 'false'`, nil)); err != nil {
						t.Fatalf("inspect persisted source state: %v", err)
					}
				})
			}
		}
	}
}

func newSourceSettingsBrowserServer(t *testing.T, savedRequests chan<- api.SaveSettingsRequest, language string) *httptest.Server {
	t.Helper()
	sources := []api.Source{
		{ID: 1, Name: "BoanNews", Host: "boannews.com", Slug: "boannews", Enabled: false},
		{ID: 2, Name: "BleepingComputer", Host: "bleepingcomputer.com", Slug: "bleepingcomputer", Enabled: true},
		{ID: 3, Name: "Cybersecurity News", Host: "cybersecuritynews.com", Slug: "cybersecuritynews", Enabled: true},
		{ID: 4, Name: "Dark Reading TI", Host: "darkreading.com", Slug: "darkreading", Enabled: true},
		{ID: 5, Name: "StepSecurity", Host: "stepsecurity.io", Slug: "stepsecurity", Enabled: true},
		{ID: 6, Name: "The Hacker News", Host: "thehackernews.com", Slug: "thehackernews", Enabled: true},
	}
	settings := api.SettingsResponse{Language: language, Accent: "#4f6ef7", LLMBaseURL: "http://localhost:11434/v1", LLMModel: "local-model", LLMTimeout: 60, TimezoneOffsetMinutes: 540}
	var mutex sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		writeJSON(t, writer, api.Bootstrap{Sources: append([]api.Source(nil), sources...), Settings: settings, Reports: []api.Report{}, CollectedDays: []string{}})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("PUT /api/settings", func(writer http.ResponseWriter, request *http.Request) {
		var value api.SaveSettingsRequest
		if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
			t.Errorf("decode settings request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		mutex.Lock()
		settings.Language = value.Language
		for _, changed := range value.Sources {
			for index := range sources {
				if sources[index].ID == changed.ID {
					sources[index].Enabled = changed.Enabled
				}
			}
		}
		response := settings
		mutex.Unlock()
		savedRequests <- value
		writeJSON(t, writer, response)
	})
	mux.Handle("/", http.FileServerFS(os.DirFS("../../../static")))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

type sourceSettingsCapture struct {
	width    int64
	language string
	theme    string
}

func captureSourceSettingsScreenshot(t *testing.T, browser context.Context, capture sourceSettingsCapture) {
	t.Helper()
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	var screenshot []byte
	if err := chromedp.Run(browser,
		chromedp.Poll(`document.querySelector('#settings-save-bar').getAnimations().every(animation => animation.playState === 'finished')`, nil),
		chromedp.Screenshot(`.app-shell`, &screenshot, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("capture source settings at %dpx: %v", capture.width, err)
	}
	writeSettingsScreenshot(t, directory, fmt.Sprintf("source-draft-%s-%s", capture.language, capture.theme), capture.width, screenshot)
}
