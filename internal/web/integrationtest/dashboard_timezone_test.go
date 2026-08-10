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

func TestDashboardWindowUsesConfiguredTimezone_atTheThirtiethDayBoundary(t *testing.T) {
	// Given the same UTC instant, where the configured offset decides which calendar day
	// "today" is and therefore where the rolling 30-day window starts.
	fixedNow := time.Date(2026, 8, 4, 9, 28, 0, 0, time.UTC)
	tests := []struct {
		name      string
		offset    int
		wantTotal int
	}{
		// UTC-12 puts today on 2026-08-03, so the window opens on 2026-07-05 and the
		// 07-05 article counts while 07-04 falls outside.
		{name: "UTC minus 12 opens the window on 07-05", offset: -12 * 60, wantTotal: 1},
		// UTC+14 puts today on 2026-08-04, moving the window to 2026-07-06 and dropping
		// the 07-05 article as well.
		{name: "UTC plus 14 opens the window on 07-06", offset: 14 * 60, wantTotal: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, feeds, appSettings := newTestServerWithConfig(t, testServerConfig{
				fetcher: &stubFetcher{},
				now:     func() time.Time { return fixedNow },
			})
			configured, err := appSettings.Get(context.Background())
			if err != nil {
				t.Fatalf("load settings: %v", err)
			}
			configured.TimezoneOffsetMinutes = test.offset
			if err := appSettings.Save(context.Background(), configured); err != nil {
				t.Fatalf("save timezone: %v", err)
			}
			for _, day := range []string{"2026-07-04", "2026-07-05"} {
				if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
					ID: "sha256:" + day, URL: "https://example.com/" + day, Title: "Boundary article",
					Description: "Boundary detail",
				}, day); err != nil {
					t.Fatalf("save article %s: %v", day, err)
				}
			}

			// When the dashboard is requested.
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))

			// Then the window boundary follows the configured offset, not SQLite's UTC clock.
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
		})
	}
}
