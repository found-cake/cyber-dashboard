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

func TestReportDeleteRequiresConfirmation_whenDeleteIsActivated(t *testing.T) {
	// Given a rendered report and a browser connected to its public API.
	server, deleteCalls, _ := newReportDeleteBrowserServer(t)
	browser := newReportDeleteBrowser(t)
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
	browser := newReportDeleteBrowser(t)
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

func newReportDeleteBrowser(t *testing.T) context.Context {
	t.Helper()
	return newBrowserContext(t, 20*time.Second)
}

func newReportDeleteBrowserServer(t *testing.T) (*httptest.Server, *atomic.Int32, <-chan struct{}) {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
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
			Settings: api.SettingsResponse{Language: "ko", TimezoneOffsetMinutes: 540},
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
