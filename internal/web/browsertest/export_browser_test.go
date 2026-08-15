//go:build browser

package browsertest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

const reportSummaryFixture = `Overview line.

■ Espionage campaign
- Patchwork(Dropping Elephant)는 Windows의 가짜 PDF 바로가기 파일과 Android의 트로이화된 채팅 앱을 이용해 정부, 국방, 에너지, 연구, 항공, 금융, 기술 조직의 사용자와 기기를 감시하고 민감한 데이터를 수집했다

■ Supply chain
- Another complete sentence.`

var exportFixtureDay, exportFixtureDisplayDay, exportFixtureNowMillis = newExportFixture(time.Now().UTC())

func newExportFixture(now time.Time) (string, string, int64) {
	return now.Format(time.DateOnly), now.Format("Monday, January 2, 2006"), now.UnixMilli()
}

func TestExportFixture_tracksRuntimeDateBeyondAugust2026(t *testing.T) {
	day, displayDay, nowMillis := newExportFixture(time.Date(2027, time.January, 2, 12, 0, 0, 0, time.UTC))
	if day != "2027-01-02" || displayDay != "Saturday, January 2, 2027" || nowMillis != 1798891200000 {
		t.Fatalf("export fixture = %q, %q, %d; want the supplied runtime date", day, displayDay, nowMillis)
	}
}

func TestPDFExportActions_openAndClosePrintWindows_fromReportAndDailyViews(t *testing.T) {
	server := newExportBrowserServer(t)
	browserContext := newExportBrowser(t)

	if err := chromedp.Run(browserContext,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-report-id="7"]`),
	); err != nil {
		t.Fatalf("open dashboard: %v", err)
	}

	var reportActionOrder string
	if err := chromedp.Run(browserContext,
		chromedp.Click(`[data-report-id="7"]`),
		chromedp.WaitVisible(`#download-report-pdf`),
		chromedp.Evaluate(`Array.from(document.querySelector('.report-sheet-actions').children).map(node => node.id).join('|')`, &reportActionOrder),
		chromedp.Evaluate(printWindowStubScript, nil),
		chromedp.Click(`#download-report-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil),
	); err != nil {
		t.Fatalf("close report print window: %v", err)
	}
	if reportActionOrder != "download-report-pdf|delete-report" {
		t.Fatalf("report action order = %q, want download-report-pdf|delete-report", reportActionOrder)
	}

	var dailyActionOrder bool
	var staleButtons int
	if err := chromedp.Run(browserContext,
		chromedp.Click(fmt.Sprintf(`.calendar-day[data-day="%s"]`, exportFixtureDay)),
		chromedp.WaitVisible(`#download-daily-pdf`),
		chromedp.Evaluate(`document.querySelectorAll('#download-daily-html, #print-daily-pdf').length`, &staleButtons),
		chromedp.Evaluate(`(() => { const container = document.querySelector('.daily-summary-actions'); return Boolean(container && container.firstElementChild?.matches('.badge') && container.lastElementChild?.id === 'download-daily-pdf'); })()`, &dailyActionOrder),
		chromedp.Evaluate(printWindowStubScript, nil),
		chromedp.Click(`#download-daily-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil),
	); err != nil {
		t.Fatalf("close daily print window: %v", err)
	}
	if staleButtons != 0 {
		t.Fatalf("stale daily export buttons = %d, want none", staleButtons)
	}
	if !dailyActionOrder {
		t.Fatal("daily action order does not place the article count before the PDF action")
	}
}

const printWindowStubScript = `(() => {
  window.__printWindowClosed = false;
  window.__printMarkup = "";
  window.open = () => {
    const printWindow = {
      document: { open() {}, write(markup) { window.__printMarkup = markup; }, close() {} },
      addEventListener(name, listener) { if (name === "afterprint") this.afterprint = listener; },
      focus() {},
      print() { if (this.afterprint) this.afterprint(); },
      close() { window.__printWindowClosed = true; }
    };
    return printWindow;
  };
})()`

func TestReportPDFDocuments_keepCoverAndDetailContent_forWeeklyAndMonthly(t *testing.T) {
	server := newExportBrowserServerWithReport(t, []string{"None", "APT28"}, reportSummaryFixture)
	for _, report := range []struct {
		id    int64
		title string
	}{{7, "Weekly report"}, {8, "Monthly report"}} {
		t.Run(report.title, func(t *testing.T) {
			markup := captureReportPDFMarkup(t, server, report.id)
			indices := []int{
				strings.Index(markup, `class="metric-grid"`),
				strings.Index(markup, `Top threat`),
				strings.Index(markup, `Key threat actors`),
				strings.Index(markup, `class="section report-summary"`),
				strings.Index(markup, `class="report-details"`),
				strings.Index(markup, `class="report-summary-details"`),
				strings.Index(markup, `Target sectors`),
			}
			for index, value := range indices {
				if value < 0 || index > 0 && value <= indices[index-1] {
					t.Fatalf("report document content order is invalid: %v", indices)
				}
			}
			if !strings.Contains(markup, report.title) || strings.Contains(markup, ">None<") || !strings.Contains(markup, "APT28") {
				t.Fatalf("report document lost its title or actor filtering: %q", markup)
			}
			if !strings.Contains(markup, `@page { size: A4; margin: 14mm; }`) || strings.Contains(markup, "■") || !strings.Contains(markup, `class="summary-item">- Patchwork`) {
				t.Fatalf("report document lost its A4 or summary formatting contract: %q", markup)
			}
			for _, tone := range []string{"metric-total", "metric-critical", "metric-high", "metric-medium"} {
				if !strings.Contains(markup, tone) {
					t.Fatalf("report document is missing %s severity styling", tone)
				}
			}
		})
	}
}

func TestReportPDFDocument_usesLocalizedFallback_whenNoActorIsIdentified(t *testing.T) {
	server := newExportBrowserServerWithReport(t, []string{"None"}, reportSummaryFixture)
	markup := captureReportPDFMarkup(t, server, 7)
	if strings.Contains(markup, ">None<") || !strings.Contains(markup, ">Unknown<") {
		t.Fatalf("report actor fallback is incorrect: %q", markup)
	}
}

func TestDailyPDFDocument_containsOnlyDailySummaryContent_inFirstPageFlow(t *testing.T) {
	server := newExportBrowserServer(t)
	markup := captureDailyPDFMarkup(t, server)
	for _, content := range []string{"Daily summary", exportFixtureDisplayDay, ">1<", "Daily threat summary", `class="section daily-summary-section"`} {
		if !strings.Contains(markup, content) {
			t.Fatalf("daily PDF document is missing %q: %q", content, markup)
		}
	}
	if strings.Contains(markup, `class="report-cover"`) || strings.Contains(markup, `class="report-details"`) || strings.Contains(markup, "Daily article") {
		t.Fatalf("daily PDF document contains report or article-detail content: %q", markup)
	}
}

func captureReportPDFMarkup(t *testing.T, server *httptest.Server, reportID int64) string {
	t.Helper()
	browserContext := newExportBrowser(t)
	selector := fmt.Sprintf(`[data-report-id="%d"]`, reportID)
	var markup string
	if err := chromedp.Run(browserContext,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
		chromedp.WaitVisible(`#download-report-pdf`),
		chromedp.Evaluate(printWindowStubScript, nil),
		chromedp.Click(`#download-report-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil),
		chromedp.Evaluate(`window.__printMarkup`, &markup),
	); err != nil {
		t.Fatalf("capture report PDF markup: %v", err)
	}
	return markup
}

func captureDailyPDFMarkup(t *testing.T, server *httptest.Server) string {
	t.Helper()
	browserContext := newExportBrowser(t)
	var markup string
	if err := chromedp.Run(browserContext,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.Click(fmt.Sprintf(`.calendar-day[data-day="%s"]`, exportFixtureDay)),
		chromedp.WaitVisible(`#download-daily-pdf`),
		chromedp.Evaluate(printWindowStubScript, nil),
		chromedp.Click(`#download-daily-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil),
		chromedp.Evaluate(`window.__printMarkup`, &markup),
	); err != nil {
		t.Fatalf("capture daily PDF markup: %v", err)
	}
	return markup
}

func newExportBrowser(t *testing.T) context.Context {
	t.Helper()
	browserContext := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browserContext, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(fmt.Sprintf("Date.now = () => %d", exportFixtureNowMillis)).Do(ctx)
		return err
	})); err != nil {
		t.Fatalf("set export browser date: %v", err)
	}
	return browserContext
}

func newExportBrowserServer(t *testing.T) *httptest.Server {
	return newExportBrowserServerWithReport(t, []string{"Unknown"}, "Weekly report summary")
}

func newExportBrowserServerWithReport(t *testing.T, actors []string, summary string) *httptest.Server {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		reports := []api.Report{{
			ID: 7, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07",
			Total: 4, Critical: 1, High: 2, Medium: 1, TopThreat: "Supply-chain intrusion",
			Actors: actors, Sectors: []string{"Technology"}, Summary: summary,
		}, {
			ID: 8, Type: "monthly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-31",
			Total: 4, Critical: 1, High: 2, Medium: 1, TopThreat: "Supply-chain intrusion",
			Actors: actors, Sectors: []string{"Technology"}, Summary: summary,
		}}
		writeJSON(t, writer, api.Bootstrap{
			Reports:  reports,
			Settings: api.SettingsResponse{Language: "en", TimezoneOffsetMinutes: 0}, CollectedDays: []string{exportFixtureDay},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("GET /api/daily/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/daily/"+exportFixtureDay {
			http.NotFound(writer, request)
			return
		}
		writeJSON(t, writer, api.Daily{Day: exportFixtureDay, Summary: "Daily threat summary", Articles: []api.Article{{
			Source: "The Hacker News", Title: "Daily article", Summary: "Article summary", URL: "https://example.com/article",
			AttackMethod: "Phishing", ThreatActor: "Unknown", Severity: "HIGH", PublishedAt: exportFixtureDay + "T09:00:00Z",
		}}})
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
