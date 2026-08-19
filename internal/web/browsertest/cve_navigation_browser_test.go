//go:build browser

package browsertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestCVEExplorerNavigatesToSubpage_whenDashboardCardIsActivated(t *testing.T) {
	// Given a dashboard with more CVEs than its compact preview can display.
	cves := make([]api.CVEInsight, 12)
	for index := range cves {
		cves[index] = api.CVEInsight{
			ID:              "CVE-2026-" + string(rune('A'+index)),
			CVSS:            9.8 - float64(index)*0.2,
			AffectedProduct: "QA product",
			FirstSeen:       "2026-08-01",
			Mentions:        index + 1,
		}
	}
	server := newCVENavigationServer(t, cves)
	browser := newBrowserContext(t, 20*time.Second)

	// When the complete CVE card is activated.
	var previewRows int
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible("#open-cve-explorer"),
		chromedp.Evaluate(`document.querySelectorAll("#main-content tbody tr").length`, &previewRows),
		chromedp.Click("#open-cve-explorer"),
		chromedp.WaitVisible(".cve-page-summary"),
	); err != nil {
		t.Fatalf("activate CVE card: %v", err)
	}

	// Then the main page shows the complete list without opening a modal.
	var result struct {
		Hash       string `json:"hash"`
		Rows       int    `json:"rows"`
		ModalItems int    `json:"modalItems"`
		ScrollTop  int    `json:"scrollTop"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`({
		hash: location.hash,
		rows: document.querySelectorAll("#main-content tbody tr").length,
		modalItems: document.querySelectorAll("#modal-root > *").length,
		scrollTop: document.querySelector("#main-content").scrollTop
	})`, &result)); err != nil {
		t.Fatalf("inspect CVE subpage: %v", err)
	}
	if previewRows != 8 {
		t.Fatalf("dashboard preview rows = %d, want 8", previewRows)
	}
	if result.Hash != "#cves" || result.Rows != len(cves) || result.ModalItems != 0 || result.ScrollTop != 0 {
		t.Fatalf("CVE subpage = %+v, want hash #cves, %d rows, no modal, and top position", result, len(cves))
	}

	// When browser history returns to the previous entry.
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`history.back()`, nil),
		chromedp.WaitVisible("#open-cve-explorer"),
	); err != nil {
		t.Fatalf("return to dashboard: %v", err)
	}

	// Then the compact dashboard preview is restored.
	var restored struct {
		Rows      int `json:"rows"`
		ScrollTop int `json:"scrollTop"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`({
		rows: document.querySelectorAll("#main-content tbody tr").length,
		scrollTop: document.querySelector("#main-content").scrollTop
	})`, &restored)); err != nil {
		t.Fatalf("inspect restored dashboard: %v", err)
	}
	if restored.Rows != 8 || restored.ScrollTop == 0 {
		t.Fatalf("restored dashboard = %+v, want 8 rows and previous scroll position", restored)
	}
}

func TestCVEExplorerRefreshesOnceAndShowsPendingCVSSAsNeutral(t *testing.T) {
	// Given the CVE explorer contains an NVD assessment that is still pending.
	server, started, release, refreshCalls := newCVERefreshServer(t)
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.WaitVisible("#refresh-cves"),
	); err != nil {
		t.Fatalf("open CVE explorer: %v", err)
	}
	var pending struct {
		Class string `json:"className"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		const badge = document.querySelector(".cve-page-table tbody tr td:nth-child(3) .badge");
		return {className: badge.className, title: badge.title, text: badge.textContent};
	})()`, &pending)); err != nil {
		t.Fatalf("inspect pending CVSS: %v", err)
	}
	if strings.Contains(pending.Class, "badge-success") || pending.Title == "" || !strings.Contains(pending.Text, "NVD") {
		t.Fatalf("pending badge = %+v, want neutral class and visible pending label", pending)
	}

	// When refresh is activated while the server request is still running.
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		const button = document.querySelector("#refresh-cves");
		button.click();
		button.click();
	})()`, nil)); err != nil {
		t.Fatalf("click refresh: %v", err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("refresh request did not reach server")
	}
	var running struct {
		Disabled bool   `json:"disabled"`
		Busy     string `json:"busy"`
		Label    string `json:"label"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		const button = document.querySelector("#refresh-cves");
		return {disabled: button.disabled, busy: button.getAttribute("aria-busy"), label: button.textContent};
	})()`, &running)); err != nil {
		t.Fatalf("inspect running refresh: %v", err)
	}
	if !running.Disabled || running.Busy != "true" || !strings.Contains(running.Label, "중") {
		t.Fatalf("running button = %+v, want disabled busy state", running)
	}

	// Then completion reloads dashboard data, unlocks the button, and reports the result.
	release()
	if err := chromedp.Run(browser,
		chromedp.WaitVisible(`tr[data-href*="CVE-2026-1001"]`),
		chromedp.WaitVisible("#toast-region .toast"),
	); err != nil {
		t.Fatalf("wait for refreshed dashboard: %v", err)
	}
	var completed struct {
		Disabled bool   `json:"disabled"`
		CVE      string `json:"cve"`
		Toast    string `json:"toast"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => ({
		disabled: document.querySelector("#refresh-cves").disabled,
		cve: document.querySelector(".cve-page-table tbody tr td:nth-child(2)").textContent,
		toast: document.querySelector("#toast-region .toast").textContent
	}))()`, &completed)); err != nil {
		t.Fatalf("inspect completed refresh: %v", err)
	}
	if refreshCalls.Load() != 1 || completed.Disabled || completed.CVE != "CVE-2026-1001" || completed.Toast == "" {
		t.Fatalf("calls = %d, completed = %+v", refreshCalls.Load(), completed)
	}
}

func newCVENavigationServer(t *testing.T, cves []api.CVEInsight) *httptest.Server {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Sources:       []api.Source{},
			Reports:       []api.ReportSummary{},
			Settings:      api.SettingsResponse{Language: "ko"},
			LLMPresets:    []api.LLMPresetResponse{},
			CollectedDays: []string{},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Total: 12, CVECount: len(cves), CVEs: cves})
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newCVERefreshServer(t *testing.T) (*httptest.Server, <-chan struct{}, func(), *atomic.Int32) {
	t.Helper()
	started := make(chan struct{}, 1)
	finish := make(chan struct{})
	var completed atomic.Bool
	var finishOnce sync.Once
	release := func() {
		finishOnce.Do(func() {
			completed.Store(true)
			close(finish)
		})
	}
	var refreshCalls atomic.Int32
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Settings: api.SettingsResponse{Language: "ko"}})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		if !completed.Load() {
			writeJSON(t, writer, api.Dashboard{CVECount: 1, CVEs: []api.CVEInsight{{
				ID: "CVE-2026-PENDING", CVSS: 0, AffectedProduct: "NVD enrichment pending", FirstSeen: "2026-08-01", Mentions: 1,
			}}})
			return
		}
		writeJSON(t, writer, api.Dashboard{CVECount: 1, CVEs: []api.CVEInsight{{
			ID: "CVE-2026-1001", CVSS: 8.1, AffectedProduct: "acme / gateway", FirstSeen: "2026-08-01", Mentions: 2,
		}}})
	})
	mux.HandleFunc("POST /api/cves/refresh", func(writer http.ResponseWriter, _ *http.Request) {
		refreshCalls.Add(1)
		started <- struct{}{}
		writeJSONStatus(t, writer, http.StatusAccepted, api.CVERefreshJob{ID: "cve-refresh-1", Status: api.CVERefreshRunning})
	})
	mux.HandleFunc("GET /api/cves/refresh/cve-refresh-1", func(writer http.ResponseWriter, _ *http.Request) {
		select {
		case <-finish:
			writeJSON(t, writer, api.CVERefreshJob{
				ID: "cve-refresh-1", Status: api.CVERefreshCompleted,
				Result: &api.CVERefreshResult{Updated: 1, Removed: 1, Warnings: []string{}},
			})
		default:
			writeJSON(t, writer, api.CVERefreshJob{ID: "cve-refresh-1", Status: api.CVERefreshRunning})
		}
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		release()
		server.Close()
	})
	return server, started, release, &refreshCalls
}

func writeJSON(t *testing.T, writer http.ResponseWriter, value any) {
	writeJSONStatus(t, writer, http.StatusOK, value)
}

func writeJSONStatus(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode browser fixture: %v", err)
	}
}
