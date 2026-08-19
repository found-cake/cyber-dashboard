//go:build browser

package browsertest

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

func TestPDFDocumentsEscapeHostileText(t *testing.T) {
	// Given report and daily fields that resemble executable markup.
	server := newExportBrowserServer(t)
	browser := newExportBrowser(t)
	payload := `<img src=x onerror='window.__exportPwned=true'> & "quoted" </section><script>window.__exportPwned=true</script>`
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode hostile fixture: %v", err)
	}
	if err := chromedp.Run(browser,
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#main-content`),
		chromedp.Evaluate(printWindowStubScript, nil),
	); err != nil {
		t.Fatalf("open export page: %v", err)
	}

	// When both exporters build print documents from the hostile values.
	var markup struct {
		Report string `json:"report"`
		Daily  string `json:"daily"`
	}
	script := fmt.Sprintf(`(() => {
		const payload = %s;
		const copy = {
			language: "en", reportWord: payload, dailySummary: payload, generatedBy: payload,
			reportTypes: {weekly: payload}, unknownActor: payload, noSummary: payload, none: payload,
			day: payload, labels: {total: payload, critical: payload, high: payload, medium: payload,
				topThreat: payload, actors: payload, summary: payload, sectors: payload, article: payload,
				articleCount: payload, date: payload}
		};
		window.cyberDashboardExport.downloadReportPDF({
			type: "weekly", period_start: payload, period_end: payload, total: 1, critical: 1, high: 0, medium: 0,
			top_threats: [{title: payload, severity: "CRITICAL"}], actors: [payload], sectors: [payload], summary: payload
		}, copy);
		const report = window.__printMarkup;
		window.cyberDashboardExport.downloadDailyPDF({summary: payload, articles: []}, copy);
		return {report, daily: window.__printMarkup};
	})()`, encodedPayload)
	if err := chromedp.Run(browser, chromedp.Evaluate(script, &markup)); err != nil {
		t.Fatalf("capture hostile export markup: %v", err)
	}

	// Then hostile values remain encoded text in both documents.
	for name, value := range map[string]string{"report": markup.Report, "daily": markup.Daily} {
		for _, raw := range []string{"<img", "<script", "</section><script"} {
			if strings.Contains(value, raw) {
				t.Fatalf("%s export contains executable fragment %q", name, raw)
			}
		}
		for _, escaped := range []string{"&lt;img", "&lt;script&gt;", "&amp;", "&quot;", "&#39;"} {
			if !strings.Contains(value, escaped) {
				t.Fatalf("%s export is missing escaped fragment %q", name, escaped)
			}
		}
	}
}
