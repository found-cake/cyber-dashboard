//go:build browser && visualqa

package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

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

func writePDFEvidence(t *testing.T, markup, outputPath string) {
	t.Helper()
	documentServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(markup))
	}))
	t.Cleanup(documentServer.Close)
	browserContext := newExportBrowser(t)

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
