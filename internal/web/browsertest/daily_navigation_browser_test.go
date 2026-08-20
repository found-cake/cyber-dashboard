//go:build browser

package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

const (
	staleDailyDay = "2026-08-12"
	newerDailyDay = "2026-08-11"
)

func TestDailyResponseStaysStale_whenDashboardOpensWhileDailyIsLoading(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
	}{
		{name: "success", status: http.StatusOK},
		{name: "failure", status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a daily request that remains pending after reaching the server.
			server, started, release := newDelayedDailyBrowserServer(t, test.status)
			browser := newBrowserContext(t, 20*time.Second)
			if err := chromedp.Run(browser,
				chromedp.ActionFunc(func(ctx context.Context) error {
					_, err := page.AddScriptToEvaluateOnNewDocument(`Date.now = () => Date.parse("2026-08-13T00:00:00Z")`).Do(ctx)
					return err
				}),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`[data-day="`+staleDailyDay+`"]`),
				chromedp.Evaluate(`window.__dailyRequestSettled = false;
					$(document).on("ajaxComplete.daily-navigation-test", (_event, _request, options) => {
						if (!options.url.endsWith("/api/daily/`+staleDailyDay+`")) return;
						requestAnimationFrame(() => requestAnimationFrame(() => { window.__dailyRequestSettled = true; }));
					})`, nil),
				chromedp.Evaluate(`document.querySelector('[data-day="`+staleDailyDay+`"]')?.click()`, nil),
			); err != nil {
				t.Fatalf("start daily request: %v", err)
			}
			waitForDailyRequest(t, started)

			// When Dashboard is opened before the delayed response settles.
			if err := chromedp.Run(browser,
				chromedp.Evaluate(`document.querySelector('[data-view="dashboard"]')?.click()`, nil),
				chromedp.WaitVisible(`#dashboard-stats`),
			); err != nil {
				t.Fatalf("open dashboard: %v", err)
			}
			release()
			if err := chromedp.Run(browser, chromedp.Poll(`window.__dailyRequestSettled === true`, nil)); err != nil {
				t.Fatalf("wait for delayed daily response: %v", err)
			}

			// Then the newer Dashboard remains visible without a stale error.
			var state struct {
				Title     string `json:"title"`
				Dashboard bool   `json:"dashboard"`
				Daily     bool   `json:"daily"`
				Errors    int    `json:"errors"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				title: document.querySelector("#page-title")?.textContent || "",
				dashboard: document.querySelector("#dashboard-stats") !== null,
				daily: document.querySelector(".article-row") !== null,
				errors: document.querySelectorAll(".toast.is-error").length
			})`, &state)); err != nil {
				t.Fatalf("inspect current view: %v", err)
			}
			if state.Title != "Dashboard" || !state.Dashboard || state.Daily || state.Errors != 0 {
				t.Fatalf("view = %+v, want Dashboard without stale daily output", state)
			}
		})
	}
}

func TestDailyResponseStaysStale_whenNewerDayFinishesFirst(t *testing.T) {
	// Given the first of two daily requests remains pending.
	server, started, release := newOutOfOrderDailyBrowserServer(t)
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`Date.now = () => Date.parse("2026-08-13T00:00:00Z")`).Do(ctx)
			return err
		}),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-day="`+staleDailyDay+`"]`),
		chromedp.Evaluate(`window.__olderDailySettled = false;
			$(document).on("ajaxComplete.daily-order-test", (_event, _request, options) => {
				if (!options.url.endsWith("/api/daily/`+staleDailyDay+`")) return;
				requestAnimationFrame(() => requestAnimationFrame(() => { window.__olderDailySettled = true; }));
			})`, nil),
		chromedp.Evaluate(`document.querySelector('[data-day="`+staleDailyDay+`"]')?.click()`, nil),
	); err != nil {
		t.Fatalf("start older daily request: %v", err)
	}
	waitForDailyRequest(t, started)

	// When the newer day renders before the first response is released.
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`document.querySelector('[data-day="`+newerDailyDay+`"]')?.click()`, nil),
		chromedp.WaitVisible(`.article-row`),
	); err != nil {
		t.Fatalf("open newer daily view: %v", err)
	}
	release()
	if err := chromedp.Run(browser,
		chromedp.Poll(`window.__olderDailySettled === true`, nil),
		chromedp.Evaluate(printWindowStubScript, nil),
		chromedp.Click(`#download-daily-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil),
	); err != nil {
		t.Fatalf("settle older response and export newer day: %v", err)
	}

	// Then the visible and exported data both belong to the newer selection.
	var state struct {
		Article string `json:"article"`
		Markup  string `json:"markup"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`({
		article: document.querySelector(".article-row h3")?.textContent || "",
		markup: window.__printMarkup
	})`, &state)); err != nil {
		t.Fatalf("inspect newer daily view: %v", err)
	}
	if state.Article != "Newer daily article" || !strings.Contains(state.Markup, "Newer daily summary") || strings.Contains(state.Markup, "Older daily summary") {
		t.Fatalf("newer daily state = %+v", state)
	}
}

func newDelayedDailyBrowserServer(t *testing.T, status int) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	started := make(chan struct{})
	released := make(chan struct{})
	var startOnce sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(released) }) }
	t.Cleanup(release)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Settings: api.SettingsResponse{Language: "en"}, CollectedDays: []string{staleDailyDay},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("GET /api/daily/"+staleDailyDay, func(writer http.ResponseWriter, _ *http.Request) {
		startOnce.Do(func() { close(started) })
		<-released
		if status != http.StatusOK {
			http.Error(writer, "daily request failed", status)
			return
		}
		writeJSON(t, writer, dailyFixture(staleDailyDay, "Delayed"))
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, started, release
}

func newOutOfOrderDailyBrowserServer(t *testing.T) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	started := make(chan struct{})
	released := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(released) }) }
	t.Cleanup(release)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Settings: api.SettingsResponse{Language: "en"}, CollectedDays: []string{staleDailyDay, newerDailyDay},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("GET /api/daily/"+staleDailyDay, func(writer http.ResponseWriter, _ *http.Request) {
		close(started)
		<-released
		writeJSON(t, writer, dailyFixture(staleDailyDay, "Older"))
	})
	mux.HandleFunc("GET /api/daily/"+newerDailyDay, func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, dailyFixture(newerDailyDay, "Newer"))
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, started, release
}

func dailyFixture(day, label string) api.Daily {
	return api.Daily{Day: day, Summary: label + " daily summary", Articles: []api.Article{{
		Source: "Archive", Title: label + " daily article", URL: "https://example.com/" + day,
		AttackMethod: "Espionage", ThreatActor: "APT", Severity: "HIGH", PublishedAt: day,
	}}}
}

func waitForDailyRequest(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("daily request did not reach server")
	}
}
