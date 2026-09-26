//go:build browser && visualqa

package browsertest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
)

func TestReportChartsPDFEvidence(t *testing.T) {
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		t.Skip("CYBER_DASHBOARD_VISUAL_QA_DIR is not configured")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	// Given fourteen category rows and ten actor rows, including a long Korean label.
	server := newRankedReportChartsBrowserServer(t)
	browser := newExportBrowser(t)
	var markup string
	// When the report's PDF download is requested through the browser.
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 1000),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-report-id="22"]`),
		chromedp.Click(`[data-report-id="22"]`),
		chromedp.WaitVisible(`.report-chart-row`, chromedp.ByQuery),
		chromedp.Evaluate(printWindowStubScript, nil),
		chromedp.Click(`#download-report-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil),
		chromedp.Evaluate(`window.__printMarkup`, &markup),
	); err != nil {
		t.Fatal(err)
	}
	// Then render the exported charts into a PDF for visual inspection.
	writePDFEvidence(t, markup, filepath.Join(directory, "export-report-charts.pdf"))
}
