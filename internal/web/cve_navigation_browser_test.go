//go:build browser

package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	browser, browserCancel := chromedp.NewContext(ctx)
	t.Cleanup(browserCancel)

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

func newCVENavigationServer(t *testing.T, cves []api.CVEInsight) *httptest.Server {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Sources:       []api.Source{},
			Reports:       []api.Report{},
			Settings:      api.Settings{Language: "ko", Theme: "dark"},
			LLMPresets:    []api.LLMPreset{},
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

func writeJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode browser fixture: %v", err)
	}
}
