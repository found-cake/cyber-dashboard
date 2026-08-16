package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
)

func TestCreateReportReturnsCreated_whenGenerationExceedsServerWriteTimeout(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
	}{
		{name: "weekly", start: "2026-08-09", end: "2026-08-15"},
		{name: "monthly", start: "2026-08-01", end: "2026-08-31"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given a real HTTP server whose normal response deadline expires during report generation.
			upstream := compatibleLLM(t, "Completed report")
			handler, feeds, appSettings := newTestServer(t, &stubFetcher{})
			configureLLM(t, appSettings, upstream.URL)
			if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
				ID: "sha256:slow-report-" + test.name, URL: "https://example.com/slow-report", Title: "Report threat",
				Description: "Report detail",
			}, "2026-08-09"); err != nil {
				t.Fatalf("save article: %v", err)
			}
			server := httptest.NewUnstartedServer(handler)
			server.Config.WriteTimeout = time.Nanosecond
			server.Start()
			t.Cleanup(server.Close)

			// When report generation completes after the server's ordinary response deadline.
			body := fmt.Sprintf(`{"type":%q,"period_start":%q,"period_end":%q}`, test.name, test.start, test.end)
			response, err := server.Client().Post(server.URL+"/api/reports", "application/json", strings.NewReader(body))

			// Then the client still receives the created report instead of an empty response.
			if err != nil {
				t.Fatalf("create report returned an empty response: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusCreated {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
			}
			var created api.Report
			if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
				t.Fatalf("decode created report: %v", err)
			}
			if created.Type != test.name || created.Summary != "Completed report" {
				t.Fatalf("created report = %+v, want type %q with completed summary", created, test.name)
			}
		})
	}
}
