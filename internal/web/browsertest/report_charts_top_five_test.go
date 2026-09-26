//go:build browser

package browsertest

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestReportCharts_unionTopRanks_whenRankingsChange(t *testing.T) {
	// Given reports with overlapping, disjoint, and unavailable prior rankings.
	server := newRankedReportChartsBrowserServer(t)
	for _, item := range []struct {
		id              int
		methods, actors []string
	}{
		{21, []string{"Increase", "Decrease", "Equal", "New", "Current fifth", "Previous top five", "Hidden", "Previous only"}, []string{"Increase", "Decrease", "Equal", "New", "Current fifth", "Previous top five", "Previous only"}},
		{22, []string{longReportActor, "Current 2", "Current 3", "Current 4", "Current 5", "Current 6", "Current 7", "Previous 1", "Previous 2", "Previous 3", "Previous 4", "Previous 5", "Previous 6", "Previous 7"}, []string{longReportActor, "Current 2", "Current 3", "Current 4", "Current 5", "Previous 1", "Previous 2", "Previous 3", "Previous 4", "Previous 5"}},
		{23, []string{"Increase", "Decrease", "Equal", "New", "Current fifth", "Previous top five", "Hidden"}, []string{"Increase", "Decrease", "Equal", "New", "Current fifth"}},
	} {
		t.Run(fmt.Sprint(item.id), func(t *testing.T) {
			browser := newExportBrowser(t)
			var panels [][]string
			// When the report is opened through the sidebar.
			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(1280, 1000), chromedp.Navigate(server.URL),
				chromedp.WaitVisible(fmt.Sprintf(`[data-report-id="%d"]`, item.id)),
				chromedp.Click(fmt.Sprintf(`[data-report-id="%d"]`, item.id)),
				chromedp.WaitVisible(`.report-chart-row`, chromedp.ByQuery),
				chromedp.Evaluate(`[...document.querySelectorAll('.report-chart-panel')].map(panel => [...panel.querySelectorAll('.report-chart-label strong')].map(label => label.textContent))`, &panels),
			); err != nil {
				t.Fatal(err)
			}
			// Then categories use each period's top seven and actors retain top five.
			if len(panels) != 2 {
				t.Fatalf("panels = %d, want 2", len(panels))
			}
			for index, want := range [][]string{item.methods, item.actors} {
				if !slices.Equal(panels[index], want) {
					t.Errorf("panel %d labels = %q, want %q", index, panels[index], want)
				}
			}
		})
	}
}

func TestReportCharts_overlayPeriods_whenCountsRiseFallOrDisappear(t *testing.T) {
	// Given increase, decrease, equal, new, and previous-only actor counts.
	server := newRankedReportChartsBrowserServer(t)
	browser := newExportBrowser(t)
	var rows []struct {
		Label                       string
		Tracks                      int
		CurrentWidth, PreviousWidth float64
		CurrentLeft, PreviousLeft   float64
		CurrentColor, PreviousColor string
		TrackWidth, MarkerLeft      float64
		ChangeLeft, ChangeWidth     float64
	}
	// When the browser lays out the comparison bars.
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 1000), chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`[data-report-id="21"]`), chromedp.Click(`[data-report-id="21"]`),
		chromedp.WaitVisible(`.report-chart-current`, chromedp.ByQuery),
		chromedp.Evaluate(`[...document.querySelectorAll('.report-chart-panel:nth-child(2) .report-chart-row')].map(row => {
			const current = row.querySelector('.report-chart-current'), previous = row.querySelector('.report-chart-previous');
			const c = current.getBoundingClientRect(), p = previous.getBoundingClientRect();
			const track = row.querySelector('.report-chart-track').getBoundingClientRect();
			const marker = row.querySelector('.report-chart-reference').getBoundingClientRect();
			const change = row.querySelector('.report-chart-change').getBoundingClientRect();
			return {label: row.querySelector('strong').textContent, tracks: row.querySelectorAll('.report-chart-track').length,
				trackWidth: track.width, markerLeft: marker.left + marker.width/2 - track.left,
				changeLeft: change.left - track.left, changeWidth: change.width,
				currentWidth: c.width, previousWidth: p.width, currentLeft: c.left, previousLeft: p.left,
				currentColor: getComputedStyle(current).backgroundColor, previousColor: getComputedStyle(previous).backgroundColor};
		})`, &rows),
	); err != nil {
		t.Fatal(err)
	}
	// Then each item has its own palette color and a lighter previous-period overlay.
	expectedRatios := map[string]float64{"Increase": 3, "Decrease": 50.0 / 70, "Equal": 1, "Previous top five": 1.0 / 6, "Previous only": 0}
	currentColors, previousColors := map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		currentColors[row.CurrentColor], previousColors[row.PreviousColor] = true, true
		if row.Tracks != 1 || math.Abs(row.CurrentLeft-row.PreviousLeft) > 0.1 || row.CurrentColor == row.PreviousColor {
			t.Errorf("comparison bars must share one origin and track with distinct period colors: %+v", row)
		}
		if ratio, ok := expectedRatios[row.Label]; ok && (row.PreviousWidth == 0 || math.Abs(row.CurrentWidth/row.PreviousWidth-ratio) > 0.01) {
			t.Errorf("%s: widths %g/%g, want ratio %g", row.Label, row.CurrentWidth, row.PreviousWidth, ratio)
		}
		if row.Label == "New" && (row.CurrentWidth <= 0 || row.PreviousWidth != 0) {
			t.Errorf("new row has incorrect visible lengths: %+v", row)
		}
		if math.Abs(row.MarkerLeft-row.PreviousWidth) > 0.1 || math.Abs(row.ChangeLeft-math.Min(row.CurrentWidth, row.PreviousWidth)) > 0.1 || math.Abs(row.ChangeWidth-math.Abs(row.CurrentWidth-row.PreviousWidth)) > 0.1 {
			t.Errorf("reference and difference must mark the exact previous value and gap: %+v", row)
		}
		if row.Label == "Previous only" && math.Abs(row.PreviousWidth-row.TrackWidth) > 0.1 {
			t.Errorf("the shared scale must include the largest prior value, 80 articles: %+v", row)
		}
	}
	if len(currentColors) != 5 || len(previousColors) != 5 {
		t.Fatalf("period palette colors = %d/%d, want five distinct colors in each period", len(currentColors), len(previousColors))
	}
}

func TestReportCharts_topRanksFitResponsiveThemes_whenRankingsAreDisjoint(t *testing.T) {
	// Given fourteen category rows and ten actor rows, including a long Korean label.
	server := newRankedReportChartsBrowserServer(t)
	for _, width := range []int64{375, 768, 1280} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			browser := newExportBrowser(t)
			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(1280, 1000), emulateColorScheme("dark"), chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`[data-report-id="22"]`), chromedp.Click(`[data-report-id="22"]`),
				chromedp.WaitVisible(`.report-chart-row`, chromedp.ByQuery),
			); err != nil {
				t.Fatal(err)
			}
			for _, theme := range []string{"dark", "light"} {
				// When the report is viewed at mobile, tablet, and desktop sizes.
				if theme == "light" {
					if err := chromedp.Run(browser, chromedp.Click(`#theme-toggle`)); err != nil {
						t.Fatal(err)
					}
				}
				var fits bool
				var appliedTheme string
				if err := chromedp.Run(browser, chromedp.EmulateViewport(width, 1000),
					chromedp.Evaluate(`document.documentElement.scrollWidth <= innerWidth && document.querySelector('.main-scroll').scrollWidth <= document.querySelector('.main-scroll').clientWidth && [...document.querySelectorAll('.report-chart-label strong')].every(label => label.scrollWidth <= label.clientWidth + 1)`, &fits),
					chromedp.Evaluate(`document.documentElement.dataset.theme`, &appliedTheme),
				); err != nil {
					t.Fatal(err)
				}
				// Then all labels fit without horizontal scrolling in either theme.
				if !fits {
					t.Fatalf("charts overflow at %dpx in %s", width, theme)
				}
				if appliedTheme != theme {
					t.Fatalf("theme = %q, want %q at %dpx", appliedTheme, theme, width)
				}
			}
		})
	}
}

func newRankedReportChartsBrowserServer(t *testing.T) *httptest.Server {
	t.Helper()
	base := newExportBrowserServer(t)
	mux := http.NewServeMux()
	current := []api.BreakdownRow{{Label: "Hidden", Value: 1}, {Label: "Increase", Value: 60}, {Label: "Decrease", Value: 50}, {Label: "Equal", Value: 40}, {Label: "New", Value: 30}, {Label: "Current fifth", Value: 20}, {Label: "Previous top five", Value: 10}}
	previous := []api.BreakdownRow{{Label: "Hidden", Value: 1}, {Label: "Increase", Value: 20}, {Label: "Decrease", Value: 70}, {Label: "Equal", Value: 40}, {Label: "Previous top five", Value: 60}, {Label: "Previous only", Value: 80}}
	charts := &api.ReportCharts{AttackMethods: current, ThreatActors: current, Previous: &api.ReportChartBaseline{ReportID: 20, PeriodStart: "2026-07-25", PeriodEnd: "2026-07-31", AttackMethods: previous, ThreatActors: previous}}
	disjoint := &api.ReportCharts{Previous: &api.ReportChartBaseline{ReportID: 20, PeriodStart: "2026-07-01", PeriodEnd: "2026-07-31"}}
	for index := 1; index <= 8; index++ {
		label := fmt.Sprintf("Current %d", index)
		if index == 1 {
			label = longReportActor
		}
		disjoint.AttackMethods = append(disjoint.AttackMethods, api.BreakdownRow{Label: label, Value: 90 - index*10})
		disjoint.Previous.AttackMethods = append(disjoint.Previous.AttackMethods, api.BreakdownRow{Label: fmt.Sprintf("Previous %d", index), Value: 90 - index*10})
	}
	disjoint.ThreatActors, disjoint.Previous.ThreatActors = disjoint.AttackMethods, disjoint.Previous.AttackMethods
	withoutBaseline := *charts
	withoutBaseline.Previous = nil
	reports := []api.Report{
		{ID: 21, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07", Charts: charts},
		{ID: 22, Type: "monthly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-31", Charts: disjoint},
		{ID: 23, Type: "weekly", PeriodStart: "2026-07-01", PeriodEnd: "2026-07-07", Charts: &withoutBaseline},
	}
	summaries := make([]api.ReportSummary, 0, len(reports))
	for _, report := range reports {
		report.Total, report.High, report.Summary = 211, 211, fmt.Sprintf("Report %d", report.ID)
		report.Actors, report.Sectors = []string{"APT28"}, []string{"Technology"}
		summaries = append(summaries, api.ReportSummary{ID: report.ID, Type: report.Type, PeriodStart: report.PeriodStart, PeriodEnd: report.PeriodEnd})
		mux.HandleFunc(fmt.Sprintf("GET /api/reports/%d", report.ID), func(writer http.ResponseWriter, _ *http.Request) { writeJSON(t, writer, report) })
	}
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Reports: summaries, Settings: api.SettingsResponse{Language: "ko"}, CollectedDays: []string{exportFixtureDay}})
	})
	mux.Handle("/", base.Config.Handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
