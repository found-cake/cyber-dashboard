//go:build browser

package browsertest

import (
	"context"
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
	server, dashboardStarted, releaseDashboard := newCVERefreshDashboardFailureServer(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	browser, browserCancel := chromedp.NewContext(ctx)
	t.Cleanup(browserCancel)

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
	server, dashboardStarted, releaseDashboard := newCVERefreshDashboardFailureServer(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	browser, browserCancel := chromedp.NewContext(ctx)
	t.Cleanup(browserCancel)

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
	); err != nil {
		t.Fatalf("supersede dashboard reload: %v", err)
	}
	releaseDashboard()

	var errors int
	if err := chromedp.Run(browser,
		chromedp.Poll(`performance.getEntriesByType("resource").filter(entry => entry.name.includes("/api/dashboard")).length === 3`, nil),
		chromedp.WaitVisible(`#dashboard-stats`),
		chromedp.Evaluate(`document.querySelectorAll('.toast.is-error').length`, &errors),
	); err != nil {
		t.Fatalf("settle superseded dashboard failure: %v", err)
	}
	if errors != 0 {
		t.Fatalf("error toasts = %d, want none from the superseded dashboard failure", errors)
	}
}

func newCVERefreshDashboardFailureServer(t *testing.T, blockFailure bool) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	dashboardStarted := make(chan struct{})
	releaseDashboard := make(chan struct{})
	var release sync.Once
	var dashboardCalls atomic.Int32
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Settings: api.SettingsResponse{Language: "en"}})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		call := dashboardCalls.Add(1)
		if call == 2 {
			close(dashboardStarted)
			if blockFailure {
				<-releaseDashboard
			}
			http.Error(writer, "dashboard refresh failed", http.StatusInternalServerError)
			return
		}
		writeJSON(t, writer, api.Dashboard{
			Total: 3,
			CVEs: []api.CVEInsight{{
				ID: "CVE-2026-1001", CVSS: 8.1, AffectedProduct: "acme / gateway", FirstSeen: "2026-08-01", Mentions: 2,
			}},
		})
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
	return server, dashboardStarted, releaseRequest
}
