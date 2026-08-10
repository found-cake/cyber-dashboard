package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
)

type blockingFetcher struct{ release <-chan struct{} }

func (f *blockingFetcher) Fetch(context.Context, api.Source) (collector.Document, error) {
	<-f.release
	return collector.Document{}, nil
}

func TestCollectRejectsRequest_whenNVDAPIKeyIsMissing(t *testing.T) {
	server, _, _ := newTestServerWithNVD(t, &stubFetcher{}, "")

	request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(`{"date":"2026-08-03"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusPreconditionFailed, recorder.Body.String())
	}
	var got api.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Code != "nvd_key_required" {
		t.Fatalf("code = %q, want nvd_key_required", got.Code)
	}
	if got.MessageKO == "" || got.MessageEN == "" {
		t.Fatalf("localized messages = %q / %q", got.MessageKO, got.MessageEN)
	}
}

func TestCollectWarnsUser_whenAIAPIIsUnavailable(t *testing.T) {
	server, _, _ := newTestServer(t, &stubFetcher{document: collector.Document{Articles: []collector.FeedArticle{{
		ID: "sha256:ai-warning", URL: "https://example.com/ai-warning", Title: "Threat",
		PublishedAt: "2026-08-03T02:00:00Z", Description: "Threat detail",
	}}}})

	request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(`{"date":"2026-08-03"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	var started api.CollectionJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	statusRequest := httptest.NewRequest(http.MethodGet, "/api/collect/"+started.ID+"?wait=1", nil)
	statusRecorder := httptest.NewRecorder()
	server.ServeHTTP(statusRecorder, statusRequest)
	var got api.CollectionJob
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode completed job: %v", err)
	}
	if got.Result == nil {
		t.Fatalf("result is nil: %+v", got)
	}
	if len(got.Result.Warnings) != 1 || got.Result.Warnings[0] != "AI API를 확인하세요 / Check the AI API" {
		t.Fatalf("warnings = %q, want bilingual AI API warning", got.Result.Warnings)
	}
}

func TestUserFacingErrorsAlwaysIncludeKoreanAndEnglishMessages(t *testing.T) {
	// Given API requests that exercise validation and repository error paths.
	server, _, _ := newTestServer(t, &stubFetcher{})
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "invalid JSON", method: http.MethodPost, path: "/api/collect", body: `{`, status: http.StatusBadRequest},
		{name: "invalid report id", method: http.MethodDelete, path: "/api/reports/not-a-number", status: http.StatusBadRequest},
		{name: "missing preset", method: http.MethodDelete, path: "/api/llm/presets/999999", status: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When the request fails at a user-facing API boundary.
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)

			// Then both localized fields and the backwards-compatible combined message are populated.
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.status, recorder.Body.String())
			}
			var got api.ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if got.MessageKO == "" || got.MessageEN == "" || got.Message != got.MessageKO+" / "+got.MessageEN {
				t.Fatalf("localized error = %+v", got)
			}
		})
	}
}

func TestCollectReturnsImmediately_beforeLongRunningWorkCompletes(t *testing.T) {
	// Given a real collection route with work blocked in the feed fetcher.
	release := make(chan struct{})
	server, _, _ := newTestServer(t, &blockingFetcher{release: release})
	t.Cleanup(func() { close(release) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(`{"date":"2026-08-03"}`))
	request.Header.Set("Content-Type", "application/json")

	// When collection starts.
	server.ServeHTTP(recorder, request)

	// Then the API acknowledges a server-owned job without waiting for collection completion.
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
}
