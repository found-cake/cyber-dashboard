//go:build browser

package browsertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

func newExportBrowserServerWithReport(t *testing.T, actors []string, summary string) *httptest.Server {
	t.Helper()
	weeklyThreats := []api.ReportThreat{
		{Title: "Supply-chain intrusion", Severity: "CRITICAL", SourceCount: 2},
		{Title: "Separate ransomware campaign", Severity: "CRITICAL", SourceCount: 1},
		{Title: "University data breach", Severity: "CRITICAL", SourceCount: 2},
		{Title: "Weekly overflow candidate", Severity: "CRITICAL", SourceCount: 1},
	}
	monthlyThreats := append(append([]api.ReportThreat{}, weeklyThreats[:3]...),
		api.ReportThreat{Title: "Cloud identity compromise", Severity: "CRITICAL", SourceCount: 1},
		api.ReportThreat{Title: "Healthcare provider extortion", Severity: "CRITICAL", SourceCount: 3},
		api.ReportThreat{Title: "Telecom credential theft", Severity: "CRITICAL", SourceCount: 1},
		api.ReportThreat{Title: "Critical infrastructure exploitation", Severity: "CRITICAL", SourceCount: 2},
		api.ReportThreat{Title: "Open-source package compromise", Severity: "CRITICAL", SourceCount: 2},
		api.ReportThreat{Title: "Financial phishing campaign", Severity: "CRITICAL", SourceCount: 1},
		api.ReportThreat{Title: "Government espionage activity", Severity: "CRITICAL", SourceCount: 2},
		api.ReportThreat{Title: "Monthly overflow candidate", Severity: "CRITICAL", SourceCount: 1},
	)
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	reports := []api.Report{{
		ID: 7, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07",
		Total: 4, Critical: 1, High: 2, Medium: 1, TopThreat: weeklyThreats[0].Title, TopThreats: weeklyThreats,
		Actors: actors, Sectors: []string{"Technology"}, Summary: summary,
	}, {
		ID: 8, Type: "monthly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-31",
		Total: 20, Critical: 8, High: 9, Medium: 3, TopThreat: monthlyThreats[0].Title, TopThreats: monthlyThreats,
		Actors: actors, Sectors: []string{"Technology"}, Summary: summary,
	}}
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		summaries := make([]api.ReportSummary, 0, len(reports))
		for _, item := range reports {
			summaries = append(summaries, api.ReportSummary{
				ID: item.ID, Type: item.Type, PeriodStart: item.PeriodStart, PeriodEnd: item.PeriodEnd,
			})
		}
		writeJSON(t, writer, api.Bootstrap{
			Reports: summaries, Settings: api.SettingsResponse{Language: "en", TimezoneOffsetMinutes: 0},
			CollectedDays: []string{exportFixtureDay},
		})
	})
	for _, item := range reports {
		mux.HandleFunc(fmt.Sprintf("GET /api/reports/%d", item.ID), func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(t, writer, item)
		})
	}
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
