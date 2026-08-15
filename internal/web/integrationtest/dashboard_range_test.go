package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
)

func TestDashboardWindowFollowsTheRequestedDayCount(t *testing.T) {
	// Given one article per window: inside all three, inside 30 and 90, and inside 90 only.
	fixedNow := time.Date(2026, 8, 4, 9, 28, 0, 0, time.UTC)
	server, feeds, _ := newTestServerWithConfig(t, testServerConfig{
		fetcher: &stubFetcher{},
		now:     func() time.Time { return fixedNow },
	})
	for _, day := range []string{"2026-08-02", "2026-07-20", "2026-06-01"} {
		if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
			ID: "sha256:" + day, URL: "https://example.com/" + day, Title: "Range article",
			Description: "Range detail",
		}, day); err != nil {
			t.Fatalf("save article %s: %v", day, err)
		}
	}
	tests := []struct {
		name          string
		query         string
		wantTotal     int
		wantOldBucket bool
	}{
		{name: "seven days opens the window on 07-29", query: "?days=7", wantTotal: 1},
		{name: "thirty days opens the window on 07-06", query: "?days=30", wantTotal: 2},
		{name: "ninety days opens the window on 05-07", query: "?days=90", wantTotal: 3, wantOldBucket: true},
		{name: "an absent range keeps the 30-day default", query: "", wantTotal: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When the dashboard is requested for that range.
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/dashboard"+test.query, nil))

			// Then only the articles inside the range are counted.
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
			}
			var value api.Dashboard
			if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
				t.Fatalf("decode dashboard: %v", err)
			}
			if value.Total != test.wantTotal {
				t.Fatalf("total = %d, want %d", value.Total, test.wantTotal)
			}
			if test.wantOldBucket && (len(value.Trend) != 10 || value.Trend[2].Start != "2026-05-25" || value.Trend[2].End != "2026-06-02" || value.Trend[2].Total != 1) {
				t.Fatalf("90-day old-article bucket = %+v", value.Trend)
			}
		})
	}
}

func TestDashboardRejectsUnsupportedDayCount(t *testing.T) {
	// Given a server and a range the dropdown never offers.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When the dashboard is requested for it.
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/dashboard?days=365", nil))

	// Then the request is rejected instead of silently aggregating another window.
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
}
