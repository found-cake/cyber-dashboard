package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCollectUsesConfiguredTimezone_whenCheckingRetentionBoundary(t *testing.T) {
	// Given the same UTC instant and collection dates around each configured day.
	fixedNow := time.Date(2026, 8, 4, 9, 28, 0, 0, time.UTC)
	tests := []struct {
		name       string
		offset     int
		day        string
		wantStatus int
	}{
		{name: "UTC minus 12 rejects the next configured day", offset: -12 * 60, day: "2026-08-04", wantStatus: http.StatusBadRequest},
		{name: "UTC plus 14 accepts the configured day", offset: 14 * 60, day: "2026-08-04", wantStatus: http.StatusPreconditionFailed},
		{name: "UTC plus 14 accepts the tenth retained day", offset: 14 * 60, day: "2026-07-26", wantStatus: http.StatusPreconditionFailed},
		{name: "UTC plus 14 rejects the eleventh day", offset: 14 * 60, day: "2026-07-25", wantStatus: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _, appSettings := newTestServerWithConfig(t, testServerConfig{
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

			// When collection is requested at the configured retention boundary.
			request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(`{"date":"`+test.day+`"}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)

			// Then the configured offset controls acceptance and the window stays ten days.
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}
