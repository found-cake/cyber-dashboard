//go:build browser

package browsertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestReportResponseStaysStale_whenDashboardOpensWhileReportIsLoading(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
	}{
		{name: "success", status: http.StatusOK},
		{name: "failure", status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a report detail request that remains pending after it reaches the server.
			server, reportStarted, releaseReport := newDelayedReportBrowserServer(t, test.status)
			browser := newBrowserContext(t, 20*time.Second)
			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(1280, 900),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`[data-report-id="7"]`),
				chromedp.Evaluate(`window.__reportRequestSettled = false;
					$(document).on("ajaxComplete.report-navigation-test", (_event, _request, options) => {
						if (!options.url.endsWith("/api/reports/7")) return;
						requestAnimationFrame(() => requestAnimationFrame(() => { window.__reportRequestSettled = true; }));
					})`, nil),
				chromedp.Click(`[data-report-id="7"]`),
			); err != nil {
				t.Fatalf("start report request: %v", err)
			}
			waitForReportRequest(t, reportStarted)

			// When Dashboard is opened before the delayed report response completes.
			if err := chromedp.Run(browser,
				chromedp.Click(`[data-view="dashboard"]`),
				chromedp.WaitVisible(`#dashboard-stats`),
			); err != nil {
				t.Fatalf("open dashboard: %v", err)
			}
			releaseReport()
			if err := chromedp.Run(browser, chromedp.Poll(`window.__reportRequestSettled === true`, nil)); err != nil {
				t.Fatalf("wait for delayed report response: %v", err)
			}

			// Then the newer Dashboard remains visible without stale report output or errors.
			var state struct {
				Title     string `json:"title"`
				Dashboard bool   `json:"dashboard"`
				Report    bool   `json:"report"`
				Errors    int    `json:"errors"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				title: document.querySelector("#page-title")?.textContent || "",
				dashboard: document.querySelector("#dashboard-stats") !== null,
				report: document.querySelector(".report-sheet") !== null,
				errors: document.querySelectorAll(".toast.is-error").length
			})`, &state)); err != nil {
				t.Fatalf("inspect current view: %v", err)
			}
			if state.Title != "Dashboard" || !state.Dashboard || state.Report || state.Errors != 0 {
				t.Fatalf("view = %+v, want Dashboard without stale report output", state)
			}
		})
	}
}

func TestReportResponseStaysStale_whenNewerReportFinishesFirst(t *testing.T) {
	// Given the first report request remains pending.
	server, reportStarted, releaseReport := newDelayedReportBrowserServer(t, http.StatusOK)
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-report-id="7"]`),
		chromedp.Evaluate(`window.__olderReportSettled = false;
			$(document).on("ajaxComplete.report-order-test", (_event, _request, options) => {
				if (!options.url.endsWith("/api/reports/7")) return;
				requestAnimationFrame(() => requestAnimationFrame(() => { window.__olderReportSettled = true; }));
			})`, nil),
		chromedp.Click(`[data-report-id="7"]`),
	); err != nil {
		t.Fatalf("start older report request: %v", err)
	}
	waitForReportRequest(t, reportStarted)

	// When the newer report renders before the first response is released.
	if err := chromedp.Run(browser,
		chromedp.Click(`[data-report-id="8"]`),
		chromedp.WaitVisible(`.report-sheet`),
	); err != nil {
		t.Fatalf("open newer report: %v", err)
	}
	releaseReport()
	if err := chromedp.Run(browser, chromedp.Poll(`window.__olderReportSettled === true`, nil)); err != nil {
		t.Fatalf("wait for older report response: %v", err)
	}

	// Then report B remains visible after report A settles.
	var summary string
	if err := chromedp.Run(browser, chromedp.Text(`.report-sheet .prose`, &summary)); err != nil {
		t.Fatalf("inspect newer report: %v", err)
	}
	if summary != "Newer report summary" {
		t.Fatalf("report summary = %q, want newer report", summary)
	}
}

func newDelayedReportBrowserServer(t *testing.T, status int) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	reportStarted := make(chan struct{})
	reportRelease := make(chan struct{})
	var startOnce sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(reportRelease) }) }
	t.Cleanup(release)

	delayed := api.Report{
		ID: 7, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07",
		Total: 4, Critical: 1, High: 2, Medium: 1, TopThreat: "Delayed report",
		Actors: []string{"Unknown"}, Sectors: []string{"Technology"}, Summary: "Delayed report summary",
	}
	newer := api.Report{
		ID: 8, Type: "weekly", PeriodStart: "2026-08-08", PeriodEnd: "2026-08-14",
		Total: 5, Critical: 2, High: 2, Medium: 1, TopThreat: "Newer report",
		Actors: []string{"APT"}, Sectors: []string{"Finance"}, Summary: "Newer report summary",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Reports: []api.ReportSummary{
				{ID: delayed.ID, Type: delayed.Type, PeriodStart: delayed.PeriodStart, PeriodEnd: delayed.PeriodEnd},
				{ID: newer.ID, Type: newer.Type, PeriodStart: newer.PeriodStart, PeriodEnd: newer.PeriodEnd},
			},
			Settings: api.SettingsResponse{Language: "en"},
		})
	})
	mux.HandleFunc("GET /api/reports/7", func(writer http.ResponseWriter, _ *http.Request) {
		startOnce.Do(func() { close(reportStarted) })
		<-reportRelease
		if status != http.StatusOK {
			writeJSONStatus(t, writer, status, api.ErrorResponse{Code: "internal", Message: "Report failed"})
			return
		}
		writeJSON(t, writer, delayed)
	})
	mux.HandleFunc("GET /api/reports/8", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, newer)
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, reportStarted, release
}

func waitForReportRequest(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("report detail request did not reach server")
	}
}
