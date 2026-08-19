//go:build browser

package browsertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestCVERefreshDashboardFailureShowsErrorWhenCurrent(t *testing.T) {
	server, dashboardStarted, _, releaseDashboard := newCVERefreshDashboardFailureServer(t, false)
	browser := newBrowserContext(t, 20*time.Second)

	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.WaitVisible(`#refresh-cves`),
		chromedp.Click(`#refresh-cves`),
	); err != nil {
		t.Fatalf("start CVE refresh: %v", err)
	}
	waitForDashboardRequest(t, dashboardStarted, "post-refresh dashboard request")

	var errors int
	if err := chromedp.Run(browser,
		chromedp.WaitVisible(`.toast.is-error`),
		chromedp.Evaluate(`document.querySelectorAll('.toast.is-error').length`, &errors),
	); err != nil {
		t.Fatalf("inspect current dashboard failure: %v", err)
	}
	if errors != 1 {
		t.Fatalf("error toasts = %d, want one for the current dashboard failure", errors)
	}
	releaseDashboard()
}

func TestCVERefreshDashboardFailureStaysSilentWhenSuperseded(t *testing.T) {
	server, dashboardStarted, dashboardFinished, releaseDashboard := newCVERefreshDashboardFailureServer(t, true)
	browser := newBrowserContext(t, 20*time.Second)

	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.WaitVisible(`#refresh-cves`),
		chromedp.Click(`#refresh-cves`),
	); err != nil {
		t.Fatalf("start CVE refresh: %v", err)
	}
	waitForDashboardRequest(t, dashboardStarted, "post-refresh dashboard request")

	if err := chromedp.Run(browser,
		chromedp.Evaluate(`window.location.hash = ""`, nil),
		chromedp.WaitVisible(`#main-content [aria-label="Loading"]`),
		chromedp.Evaluate(`window.__supersededDashboardSettled = false;
			$(document).on("ajaxComplete.cve-dashboard-failure-test", (_event, request, options) => {
				if (!options.url.endsWith("/api/dashboard?days=30") || request.getResponseHeader("X-Test-Delayed-Dashboard") !== "1") return;
				requestAnimationFrame(() => requestAnimationFrame(() => { window.__supersededDashboardSettled = true; }));
			})`, nil),
	); err != nil {
		t.Fatalf("supersede dashboard reload: %v", err)
	}
	releaseDashboard()
	waitForDashboardRequest(t, dashboardFinished, "superseded dashboard failure completion")

	var errors int
	if err := chromedp.Run(browser,
		chromedp.Poll(`window.__supersededDashboardSettled === true`, nil),
		chromedp.WaitVisible(`#dashboard-stats`),
		chromedp.Evaluate(`document.querySelectorAll('.toast.is-error').length`, &errors),
	); err != nil {
		t.Fatalf("settle superseded dashboard failure: %v", err)
	}
	if errors != 0 {
		t.Fatalf("error toasts = %d, want none from the superseded dashboard failure", errors)
	}
}

func newCVERefreshDashboardFailureServer(t *testing.T, blockFailure bool) (*httptest.Server, <-chan struct{}, <-chan struct{}, func()) {
	t.Helper()
	dashboardStarted := make(chan struct{})
	dashboardFinished := make(chan struct{})
	releaseDashboard := make(chan struct{})
	var release sync.Once
	var finish sync.Once
	var dashboardCalls atomic.Int32
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Settings: api.SettingsResponse{Language: "en"}})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		call := dashboardCalls.Add(1)
		if call == 1 {
			writer.Header().Set("X-Test-Delayed-Dashboard", "1")
			close(dashboardStarted)
			if blockFailure {
				<-releaseDashboard
			}
			http.Error(writer, "dashboard refresh failed", http.StatusInternalServerError)
			finish.Do(func() { close(dashboardFinished) })
			return
		}
		writeJSON(t, writer, api.Dashboard{
			Total: 3,
			CVEs: []api.CVEInsight{{
				ID: "CVE-2026-1001", CVSS: 8.1, AffectedProduct: "acme / gateway", FirstSeen: "2026-08-01", Mentions: 2,
			}},
		})
	})
	mux.HandleFunc("GET /api/cves", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, []api.CVEInsight{{
			ID: "CVE-2026-1001", CVSS: 8.1, AffectedProduct: "acme / gateway", FirstSeen: "2026-08-01", Mentions: 2,
		}})
	})
	mux.HandleFunc("POST /api/cves/refresh", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSONStatus(t, writer, http.StatusAccepted, api.CVERefreshJob{
			ID: "cve-refresh-1", Status: api.CVERefreshCompleted,
			Result: &api.CVERefreshResult{Updated: 1, Removed: 0},
		})
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	releaseRequest := func() { release.Do(func() { close(releaseDashboard) }) }
	t.Cleanup(releaseRequest)
	return server, dashboardStarted, dashboardFinished, releaseRequest
}
