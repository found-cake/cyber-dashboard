package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
)

func TestReportChartsPreserveGeneratedCountsThroughHTTP(t *testing.T) {
	// Given two reportable periods and the real application HTTP server.
	upstream := compatibleLLM(t, "Report summary")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	for _, day := range []string{"2026-08-01", "2026-08-08"} {
		if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
			ID: day, URL: "https://example.com/" + day, Title: "Incident " + day, Description: "Report facts",
		}, day); err != nil {
			t.Fatal(err)
		}
		if err := feeds.SaveDailySummary(context.Background(), day, "Daily digest"); err != nil {
			t.Fatal(err)
		}
	}
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	create := func(kind, start, end string) api.Report {
		t.Helper()
		body := fmt.Sprintf(`{"type":%q,"period_start":%q,"period_end":%q}`, kind, start, end)
		response, err := httpServer.Client().Post(httpServer.URL+"/api/reports", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create report status = %d", response.StatusCode)
		}
		var value api.Report
		if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	// When weekly reports are generated successively and the later one is fetched.
	previous := create("weekly", "2026-08-01", "2026-08-07")
	current := create("weekly", "2026-08-08", "2026-08-14")
	if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
		ID: "late-article", URL: "https://example.com/late", Title: "Late collected incident", Description: "New report facts",
	}, "2026-08-08"); err != nil {
		t.Fatal(err)
	}
	response, err := httpServer.Client().Get(fmt.Sprintf("%s/api/reports/%d", httpServer.URL, current.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var stored api.Report
	if err := json.NewDecoder(response.Body).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	// Then the API preserves generated counts and resolves the previous report's period metadata.
	if stored.Charts == nil || stored.Charts.Previous == nil || stored.Charts.Previous.ReportID != previous.ID {
		t.Fatalf("stored charts = %+v", stored.Charts)
	}
	if stored.Charts.AttackMethods[0].Value != 1 || current.Charts.AttackMethods[0].Value != 1 || stored.Charts.Previous.AttackMethods[0].Value != 1 {
		t.Fatalf("stored charts = %+v, current = %+v", stored.Charts, current.Charts)
	}
	monthly := create("monthly", "2026-08-01", "2026-08-31")
	if monthly.Charts == nil || monthly.Charts.Previous != nil || monthly.Charts.AttackMethods[0].Value != 3 {
		t.Fatalf("monthly charts = %+v", monthly.Charts)
	}
}
