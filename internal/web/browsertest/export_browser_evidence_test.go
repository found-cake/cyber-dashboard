//go:build browser

package browsertest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type exportEvidenceWriter struct {
	directory string
}

func TestExportBrowserEvidence(t *testing.T) {
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		t.Skip("CYBER_DASHBOARD_VISUAL_QA_DIR is not configured")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	writer := exportEvidenceWriter{directory: directory}

	for _, viewport := range []struct {
		width  int64
		height int64
	}{{375, 812}, {768, 900}, {1280, 900}} {
		t.Run(fmt.Sprintf("%dpx", viewport.width), func(t *testing.T) {
			server := newExportBrowserServer(t)
			browserContext, cancel := newExportBrowser(t)
			defer cancel()
			if err := chromedp.Run(browserContext,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`[data-report-id="7"]`),
			); err != nil {
				t.Fatalf("open dashboard: %v", err)
			}
			if viewport.width <= 900 {
				openExportNavigation(t, browserContext)
			}
			if err := chromedp.Run(browserContext,
				chromedp.Click(`[data-report-id="7"]`),
				chromedp.WaitVisible(`#download-report-pdf`),
			); err != nil {
				t.Fatalf("open report: %v", err)
			}
			writer.capture(t, browserContext, fmt.Sprintf("export-report-%d.png", viewport.width))

			if viewport.width <= 900 {
				openExportNavigation(t, browserContext)
			}
			if err := chromedp.Run(browserContext,
				chromedp.Click(fmt.Sprintf(`.calendar-day[data-day="%s"]`, exportFixtureDay)),
				chromedp.WaitVisible(`#download-daily-pdf`),
			); err != nil {
				t.Fatalf("open daily summary: %v", err)
			}
			writer.capture(t, browserContext, fmt.Sprintf("export-daily-%d.png", viewport.width))
		})
	}
}

func TestExportPDFEvidence(t *testing.T) {
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		t.Skip("CYBER_DASHBOARD_VISUAL_QA_DIR is not configured")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	server := newExportBrowserServerWithReport(t, []string{"None", "APT28"}, reportSummaryFixture)
	writePDFEvidence(t, captureReportPDFMarkup(t, server, 7), filepath.Join(directory, "export-weekly.pdf"))
	writePDFEvidence(t, captureReportPDFMarkup(t, server, 8), filepath.Join(directory, "export-monthly.pdf"))
	writePDFEvidence(t, captureDailyPDFMarkup(t, server), filepath.Join(directory, "export-daily.pdf"))
}

func openExportNavigation(t *testing.T, browserContext context.Context) {
	t.Helper()
	if err := chromedp.Run(browserContext,
		chromedp.Click(`#menu-button`),
		chromedp.Poll(`Math.abs(document.querySelector('#sidebar').getBoundingClientRect().left) < 1`, nil),
	); err != nil {
		t.Fatalf("open navigation: %v", err)
	}
}

func (writer exportEvidenceWriter) capture(t *testing.T, browserContext context.Context, filename string) {
	t.Helper()
	var screenshot []byte
	if err := chromedp.Run(browserContext, chromedp.CaptureScreenshot(&screenshot)); err != nil {
		t.Fatalf("capture %s: %v", filename, err)
	}
	if err := os.WriteFile(filepath.Join(writer.directory, filename), screenshot, 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}

func writePDFEvidence(t *testing.T, markup, outputPath string) {
	t.Helper()
	documentServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(markup))
	}))
	t.Cleanup(documentServer.Close)
	browserContext, cancel := newExportBrowser(t)
	defer cancel()

	var document []byte
	if err := chromedp.Run(browserContext,
		chromedp.Navigate(documentServer.URL),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			document, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				Do(ctx)
			return err
		}),
	); err != nil {
		t.Fatalf("render %s: %v", outputPath, err)
	}
	if err := os.WriteFile(outputPath, document, 0o644); err != nil {
		t.Fatalf("write %s: %v", outputPath, err)
	}
}
