//go:build browser

package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestCVEExplorerResumesActiveRefreshAfterReload_withoutAnotherPost(t *testing.T) {
	server, release, bootstrapCalls, posts, polls := newCVEResumeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	browser, browserCancel := chromedp.NewContext(ctx)
	t.Cleanup(browserCancel)

	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(768, 900),
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.WaitVisible("#refresh-cves"),
	); err != nil {
		t.Fatalf("open active CVE refresh: %v", err)
	}
	waitForRefreshPoll(t, polls, 1)
	assertCVERefreshBusy(t, browser)

	if err := chromedp.Run(browser,
		chromedp.Reload(),
		chromedp.WaitVisible("#refresh-cves"),
	); err != nil {
		t.Fatalf("reload CVE explorer: %v", err)
	}
	waitForRefreshPoll(t, polls, 2)
	assertCVERefreshBusy(t, browser)
	if bootstrapCalls.Load() < 2 || posts.Load() != 0 {
		t.Fatalf("bootstrap calls = %d, refresh POSTs = %d", bootstrapCalls.Load(), posts.Load())
	}

	release()
	if err := chromedp.Run(browser,
		chromedp.WaitVisible(`tr[data-href*="CVE-2026-RESUMED"]`),
		chromedp.WaitVisible("#toast-region .toast"),
	); err != nil {
		t.Fatalf("wait for resumed refresh completion: %v", err)
	}
	var disabled bool
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelector("#refresh-cves").disabled`, &disabled)); err != nil {
		t.Fatalf("inspect completed refresh: %v", err)
	}
	if disabled || posts.Load() != 0 {
		t.Fatalf("button disabled = %v, refresh POSTs = %d", disabled, posts.Load())
	}
}

func newCVEResumeServer(t *testing.T) (*httptest.Server, func(), *atomic.Int32, *atomic.Int32, <-chan int32) {
	t.Helper()
	var completed atomic.Bool
	var bootstrapCalls atomic.Int32
	var posts atomic.Int32
	polls := make(chan int32, 16)
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		bootstrapCalls.Add(1)
		writeJSON(t, writer, api.Bootstrap{
			Settings: api.SettingsResponse{Language: "ko"},
			CVERefresh: &api.CVERefreshJob{
				ID: "cve-refresh-resume", Status: api.CVERefreshRunning,
			},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		id := "CVE-2026-PENDING"
		if completed.Load() {
			id = "CVE-2026-RESUMED"
		}
		writeJSON(t, writer, api.Dashboard{CVECount: 1, CVEs: []api.CVEInsight{{
			ID: id, CVSS: 7.5, AffectedProduct: "resume fixture", FirstSeen: "2026-08-02", Mentions: 1,
		}}})
	})
	mux.HandleFunc("POST /api/cves/refresh", func(writer http.ResponseWriter, _ *http.Request) {
		posts.Add(1)
		writeJSONStatus(t, writer, http.StatusAccepted, api.CVERefreshJob{ID: "unexpected", Status: api.CVERefreshRunning})
	})
	mux.HandleFunc("GET /api/cves/refresh/cve-refresh-resume", func(writer http.ResponseWriter, _ *http.Request) {
		select {
		case polls <- bootstrapCalls.Load():
		default:
		}
		job := api.CVERefreshJob{ID: "cve-refresh-resume", Status: api.CVERefreshRunning}
		if completed.Load() {
			job.Status = api.CVERefreshCompleted
			job.Result = &api.CVERefreshResult{Updated: 1, Warnings: []string{}}
		}
		writeJSON(t, writer, job)
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, func() { completed.Store(true) }, &bootstrapCalls, &posts, polls
}

func waitForRefreshPoll(t *testing.T, polls <-chan int32, minimumBootstrapCalls int32) {
	t.Helper()
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	for {
		select {
		case calls := <-polls:
			if calls >= minimumBootstrapCalls {
				return
			}
		case <-timer.C:
			t.Fatalf("refresh was not polled after %d bootstrap requests", minimumBootstrapCalls)
		}
	}
}

func assertCVERefreshBusy(t *testing.T, browser context.Context) {
	t.Helper()
	var state struct {
		Disabled bool   `json:"disabled"`
		Busy     string `json:"busy"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		const button = document.querySelector("#refresh-cves");
		return {disabled: button.disabled, busy: button.getAttribute("aria-busy")};
	})()`, &state)); err != nil {
		t.Fatalf("inspect active refresh: %v", err)
	}
	if !state.Disabled || state.Busy != "true" {
		t.Fatalf("active refresh button = %+v", state)
	}
}
