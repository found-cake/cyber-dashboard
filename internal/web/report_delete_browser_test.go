//go:build browser

package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestReportDeleteRequiresConfirmation_whenDeleteIsActivated(t *testing.T) {
	// Given a rendered report and a browser connected to its public API.
	server, deleteCalls, _ := newReportDeleteBrowserServer(t)
	browser, cancel := newReportDeleteBrowser(t)
	defer cancel()
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-report-id="7"]`),
		chromedp.Click(`[data-report-id="7"]`),
		chromedp.WaitVisible(`.report-sheet`),
	); err != nil {
		t.Fatalf("open report: %v", err)
	}

	// When the report delete action is activated once.
	if err := chromedp.Run(browser,
		chromedp.Click(`#delete-report`),
		chromedp.WaitVisible(`#confirm-delete-report`),
	); err != nil {
		t.Fatalf("open delete confirmation: %v", err)
	}

	// Then the confirmation dialog is visible without sending a delete request.
	var state struct {
		Dialog      bool `json:"dialog"`
		ReportEntry bool `json:"reportEntry"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`({
		dialog: document.querySelector('#modal-root [role="dialog"]') !== null,
		reportEntry: document.querySelector('[data-report-id="7"]') !== null
	})`, &state)); err != nil {
		t.Fatalf("inspect confirmation: %v", err)
	}
	if deleteCalls.Load() != 0 || !state.Dialog || !state.ReportEntry {
		t.Fatalf("delete calls = %d, state = %+v, want untouched report behind confirmation", deleteCalls.Load(), state)
	}
}

func TestReportDeleteRemovesReport_whenConfirmationIsAccepted(t *testing.T) {
	// Given an open confirmation dialog for a stored report.
	server, deleteCalls, deleteRequested := newReportDeleteBrowserServer(t)
	browser, cancel := newReportDeleteBrowser(t)
	defer cancel()
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-report-id="7"]`),
		chromedp.Click(`[data-report-id="7"]`),
		chromedp.WaitVisible(`#delete-report`),
		chromedp.Click(`#delete-report`),
		chromedp.WaitVisible(`#confirm-delete-report`),
	); err != nil {
		t.Fatalf("open delete confirmation: %v", err)
	}

	// When deletion is confirmed.
	if err := chromedp.Run(browser, chromedp.Click(`#confirm-delete-report`)); err != nil {
		t.Fatalf("activate deletion confirmation: %v", err)
	}
	select {
	case <-deleteRequested:
	case <-time.After(3 * time.Second):
		t.Fatal("delete request did not reach server")
	}
	if err := chromedp.Run(browser, chromedp.WaitNotPresent(`[data-report-id="7"]`)); err != nil {
		t.Fatalf("wait for removed report navigation: %v", err)
	}
	if err := chromedp.Run(browser, chromedp.WaitVisible(`#toast-region .toast`)); err != nil {
		t.Fatalf("wait for deletion toast: %v", err)
	}
	if err := chromedp.Run(browser, chromedp.WaitVisible(`#main-content .empty-state`)); err != nil {
		t.Fatalf("wait for dashboard empty state: %v", err)
	}

	// Then one request is sent and the report disappears from navigation and content.
	var modalItems int
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelectorAll('#modal-root > *').length`, &modalItems)); err != nil {
		t.Fatalf("inspect completed deletion: %v", err)
	}
	if deleteCalls.Load() != 1 || modalItems != 0 {
		t.Fatalf("delete calls = %d, modal items = %d, want one request and closed dialog", deleteCalls.Load(), modalItems)
	}
}

func TestReportDeleteVisualEvidence(t *testing.T) {
	type browserStage struct {
		name    string
		actions []chromedp.Action
	}

	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		t.Skip("CYBER_DASHBOARD_VISUAL_QA_DIR is not configured")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	for _, viewport := range []struct {
		width  int64
		height int64
	}{{375, 812}, {768, 900}, {1280, 900}} {
		server, _, _ := newReportDeleteBrowserServer(t)
		browser, cancel := newReportDeleteBrowser(t)
		stages := []browserStage{{"navigate", []chromedp.Action{
			chromedp.EmulateViewport(viewport.width, viewport.height), chromedp.Navigate(server.URL),
		}}}
		if viewport.width <= 900 {
			stages = append(stages, browserStage{"open navigation", []chromedp.Action{
				chromedp.WaitVisible(`#menu-button`),
				chromedp.Click(`#menu-button`),
				chromedp.Poll(`Math.abs(document.querySelector('#sidebar').getBoundingClientRect().left) < 1`, nil),
			}})
		}
		stages = append(stages,
			browserStage{"open report", []chromedp.Action{chromedp.WaitVisible(`[data-report-id="7"]`), chromedp.Click(`[data-report-id="7"]`)}},
			browserStage{"find delete action", []chromedp.Action{chromedp.WaitVisible(`#delete-report`)}},
			browserStage{"activate delete action", []chromedp.Action{chromedp.Click(`#delete-report`)}},
			browserStage{"find confirmation", []chromedp.Action{chromedp.WaitVisible(`#confirm-delete-report`)}},
			browserStage{"settle confirmation", []chromedp.Action{chromedp.Poll(`document.querySelector('.modal').getAnimations().every(animation => animation.playState === 'finished')`, nil)}},
		)
		for _, stage := range stages {
			if err := chromedp.Run(browser, stage.actions...); err != nil {
				cancel()
				t.Fatalf("%s at %dpx: %v", stage.name, viewport.width, err)
			}
		}
		var screenshot []byte
		if err := chromedp.Run(browser, chromedp.CaptureScreenshot(&screenshot)); err != nil {
			cancel()
			t.Fatalf("capture delete confirmation at %dpx: %v", viewport.width, err)
		}
		cancel()
		path := filepath.Join(directory, fmt.Sprintf("report-delete-confirmation-%d.png", viewport.width))
		if err := os.WriteFile(path, screenshot, 0o644); err != nil {
			t.Fatalf("write delete confirmation at %dpx: %v", viewport.width, err)
		}
	}
}

func newReportDeleteBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	browser, browserCancel := chromedp.NewContext(ctx)
	return browser, func() {
		browserCancel()
		cancel()
	}
}

func newReportDeleteBrowserServer(t *testing.T) (*httptest.Server, *atomic.Int32, <-chan struct{}) {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../static"))
	var deleteCalls atomic.Int32
	deleteRequested := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Reports: []api.Report{{
				ID: 7, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07",
				Total: 4, Critical: 1, High: 2, Medium: 1, TopThreat: "Supply-chain intrusion",
				Actors: []string{"Unknown"}, Sectors: []string{"Technology"}, Summary: "Weekly report summary",
			}},
			Settings: api.SettingsResponse{Language: "ko", Theme: "dark", TimezoneOffsetMinutes: 540},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("DELETE /api/reports/7", func(writer http.ResponseWriter, _ *http.Request) {
		deleteCalls.Add(1)
		deleteRequested <- struct{}{}
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &deleteCalls, deleteRequested
}
