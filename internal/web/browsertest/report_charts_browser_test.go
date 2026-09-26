//go:build browser

package browsertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

const longReportActor = "한글 위협 행위자 VeryLongUnbrokenThreatActorNameThatMustRemainVisibleWithoutHorizontalOverflow"

func TestReportCharts_compareSavedReports_whenWeeklyOrMonthlyOpened(t *testing.T) {
	for _, language := range []string{"en", "ko"} {
		t.Run(language, func(t *testing.T) {
			// Given both report types contain a saved comparison and long actor labels.
			server := newReportChartsBrowserServer(t, language)
			browser := newExportBrowser(t)
			if err := chromedp.Run(browser, chromedp.EmulateViewport(1280, 1000), chromedp.Navigate(server.URL), chromedp.WaitVisible(`[data-report-id="7"]`)); err != nil {
				t.Fatal(err)
			}
			for _, id := range []int{7, 8} {
				// When navigating from one report to the other through the sidebar.
				var content, markup string
				if err := chromedp.Run(browser,
					chromedp.Click(fmt.Sprintf(`[data-report-id="%d"]`, id)),
					chromedp.Poll(fmt.Sprintf(`document.querySelector(".report-sheet .prose")?.textContent.includes("Report %d")`, id), nil),
					chromedp.Text(`.report-charts`, &content, chromedp.ByQuery),
					chromedp.Evaluate(printWindowStubScript, nil),
					chromedp.Click(`#download-report-pdf`),
					chromedp.Poll(`window.__printWindowClosed === true`, nil),
					chromedp.Evaluate(`window.__printMarkup`, &markup),
				); err != nil {
					t.Fatal(err)
				}
				// Then the current and previous-only categories retain their exact deltas in both surfaces.
				want := []string{"Phishing", "Ransomware", "Malware", "75.0%", "25.0%", "+50.0", "−75.0", "APT28", "+2", "−2", longReportActor}
				if language == "ko" {
					want = append(want, "위협 유형 분포", "위협 행위자 분포", "%p", "신규", "미관측")
				} else {
					want = append(want, "Threat category breakdown", "Threat actor breakdown", "pp", "New", "Not observed")
				}
				for _, value := range want {
					if !strings.Contains(content, value) || !strings.Contains(markup, value) {
						t.Errorf("report %d missing %q in visible chart or PDF markup", id, value)
					}
				}
			}
		})
	}
}

func TestReportCharts_missingPeriodDataAndDaily_doNotInventComparisons(t *testing.T) {
	// Given a first report, a report without period articles, and a daily summary.
	server := newReportChartsBrowserServer(t, "en")
	browser := newExportBrowser(t)
	if err := chromedp.Run(browser, chromedp.EmulateViewport(1280, 900), chromedp.Navigate(server.URL), chromedp.WaitVisible(`[data-report-id="9"]`)); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, rows int
		message  string
	}{{9, 4, "No comparison data"}, {10, 0, "No article data"}} {
		// When opening a report whose comparison or current snapshot was never saved.
		var content string
		var rows, deltas int
		if err := chromedp.Run(browser,
			chromedp.Click(fmt.Sprintf(`[data-report-id="%d"]`, item.id)),
			chromedp.Poll(fmt.Sprintf(`document.querySelector(".report-sheet .prose")?.textContent.includes("Report %d")`, item.id), nil),
			chromedp.Text(`.report-charts`, &content, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelectorAll(".report-chart-row").length`, &rows),
			chromedp.Evaluate(`document.querySelectorAll(".report-chart-delta").length`, &deltas),
		); err != nil {
			t.Fatal(err)
		}
		// Then absence is explained and no fabricated deltas appear.
		if rows != item.rows || deltas != 0 || !strings.Contains(content, item.message) {
			t.Fatalf("report %d: rows=%d deltas=%d content=%q", item.id, rows, deltas, content)
		}
	}
	var dailyCharts int
	var markup string
	if err := chromedp.Run(browser,
		chromedp.Click(fmt.Sprintf(`.calendar-day[data-day="%s"]`, exportFixtureDay)),
		chromedp.WaitVisible(`#download-daily-pdf`),
		chromedp.Evaluate(`document.querySelectorAll(".report-charts").length`, &dailyCharts),
		chromedp.Evaluate(printWindowStubScript, nil), chromedp.Click(`#download-daily-pdf`),
		chromedp.Poll(`window.__printWindowClosed === true`, nil), chromedp.Evaluate(`window.__printMarkup`, &markup),
	); err != nil {
		t.Fatal(err)
	}
	if dailyCharts != 0 || strings.Contains(markup, `class="report-charts"`) {
		t.Fatal("daily screen or export unexpectedly contains report charts")
	}
}

func newReportChartsBrowserServer(t *testing.T, language string) *httptest.Server {
	t.Helper()
	base := newExportBrowserServer(t)
	mux := http.NewServeMux()
	charts := &api.ReportCharts{
		AttackMethods: []api.BreakdownRow{{Label: "Phishing", Value: 3}, {Label: "Ransomware", Value: 1}},
		ThreatActors:  []api.BreakdownRow{{Label: "APT28", Value: 3}, {Label: longReportActor, Value: 1}},
		Previous: &api.ReportChartBaseline{ReportID: 6, PeriodStart: "2026-07-25", PeriodEnd: "2026-07-31",
			AttackMethods: []api.BreakdownRow{{Label: "Phishing", Value: 1}, {Label: "Malware", Value: 3}},
			ThreatActors:  []api.BreakdownRow{{Label: "APT28", Value: 1}, {Label: "Old actor", Value: 2}}},
	}
	withoutBaseline := *charts
	withoutBaseline.Previous = nil
	reports := []api.Report{
		{ID: 7, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07", Charts: charts},
		{ID: 8, Type: "monthly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-31", Charts: charts},
		{ID: 9, Type: "weekly", PeriodStart: "2026-07-01", PeriodEnd: "2026-07-07", Charts: &withoutBaseline},
		{ID: 10, Type: "monthly", PeriodStart: "2026-06-01", PeriodEnd: "2026-06-30"},
	}
	summaries := make([]api.ReportSummary, 0, len(reports))
	for _, report := range reports {
		report.Total, report.High, report.Summary = 4, 4, fmt.Sprintf("Report %d", report.ID)
		report.Actors, report.Sectors = []string{"APT28"}, []string{"Technology"}
		summaries = append(summaries, api.ReportSummary{ID: report.ID, Type: report.Type, PeriodStart: report.PeriodStart, PeriodEnd: report.PeriodEnd})
		mux.HandleFunc(fmt.Sprintf("GET /api/reports/%d", report.ID), func(writer http.ResponseWriter, _ *http.Request) { writeJSON(t, writer, report) })
	}
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Reports: summaries, Settings: api.SettingsResponse{Language: language}, CollectedDays: []string{exportFixtureDay}})
	})
	mux.Handle("/", base.Config.Handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
