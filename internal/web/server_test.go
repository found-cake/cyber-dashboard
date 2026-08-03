package web_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/feed"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/web"
)

func TestDashboardStartsEmpty_whenDatabaseIsNew(t *testing.T) {
	// Given a new database and the real HTTP router.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When the dashboard API is requested.
	request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the first-run state is explicit and contains no fabricated metrics.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var got api.Dashboard
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if got.Total != 0 || !got.Empty {
		t.Fatalf("dashboard = %+v, want empty zero state", got)
	}
}

func TestCollectDayDeduplicatesFeedArticles_whenRepeated(t *testing.T) {
	// Given an enabled feed that returns one article for the requested day.
	server, feeds, _ := newTestServer(t, &stubFetcher{document: feed.Document{Articles: []feed.FeedArticle{{
		ID: "sha256:stable-id", URL: "https://example.com/article",
		Title: "CVE-2026-1547 exploited in the wild", PublishedAt: "2026-08-03T01:00:00Z",
		Description: "Attackers exploit CVE-2026-1547 against public servers.",
		Categories:  []string{"Vulnerability"},
	}}}})
	body := `{"date":"2026-08-03"}`

	// When the same collection request runs twice.
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("collect status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	}

	// Then feed_uid remains the deduplication key and CVEs are regex-derived.
	daily, err := feeds.Daily(context.Background(), "2026-08-03")
	if err != nil {
		t.Fatalf("load daily: %v", err)
	}
	if len(daily.Articles) != 1 {
		t.Fatalf("article count = %d, want 1", len(daily.Articles))
	}
	if len(daily.Articles[0].CVEs) != 1 || daily.Articles[0].CVEs[0] != "CVE-2026-1547" {
		t.Fatalf("CVEs = %v, want CVE-2026-1547", daily.Articles[0].CVEs)
	}
}

func TestCollectGeneratesDailySummary_whenLLMIsConfigured(t *testing.T) {
	// Given a configured compatible LLM and one article for the requested day.
	upstream := compatibleLLM(t, "일간 보안 요약")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{document: feed.Document{Articles: []feed.FeedArticle{{
		ID: "sha256:daily-summary", URL: "https://example.com/daily",
		Title: "Daily threat", PublishedAt: "2026-08-03T02:00:00Z", Description: "Threat detail",
	}}}})
	configureLLM(t, appSettings, upstream.URL)

	// When collection completes.
	request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(`{"date":"2026-08-03"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the SDK result is persisted as the daily summary.
	if recorder.Code != http.StatusOK {
		t.Fatalf("collect status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	daily, err := feeds.Daily(context.Background(), "2026-08-03")
	if err != nil {
		t.Fatalf("load daily: %v", err)
	}
	if daily.Summary != "일간 보안 요약" {
		t.Fatalf("summary = %q, want 일간 보안 요약", daily.Summary)
	}
}

func TestCreateReportGeneratesSummary_whenLLMIsConfigured(t *testing.T) {
	// Given collected article data and a configured compatible LLM.
	upstream := compatibleLLM(t, "주간 보안 요약")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, feed.FeedArticle{
		ID: "sha256:report", URL: "https://example.com/report", Title: "Weekly threat",
		Description: "Report detail",
	}, "2026-08-02"); err != nil {
		t.Fatalf("save article: %v", err)
	}

	// When a weekly report is created.
	request := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(
		`{"type":"weekly","period_start":"2026-08-01","period_end":"2026-08-03"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the report contains the SDK-generated summary.
	if recorder.Code != http.StatusCreated {
		t.Fatalf("report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var got api.Report
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if got.Summary != "주간 보안 요약" {
		t.Fatalf("summary = %q, want 주간 보안 요약", got.Summary)
	}
}

func TestLLMPresetAPIManagesOnlyUserPresets(t *testing.T) {
	// Given a new server with the built-in OpenAI preset.
	server, _, _ := newTestServer(t, &stubFetcher{})
	bootstrapRequest := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	bootstrapRecorder := httptest.NewRecorder()
	server.ServeHTTP(bootstrapRecorder, bootstrapRequest)
	var bootstrap api.Bootstrap
	if err := json.Unmarshal(bootstrapRecorder.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if len(bootstrap.LLMPresets) != 1 || !bootstrap.LLMPresets[0].Builtin {
		t.Fatalf("bootstrap presets = %+v, want built-in OpenAI", bootstrap.LLMPresets)
	}

	// When a compatible endpoint is added through the public API.
	createRequest := httptest.NewRequest(http.MethodPost, "/api/llm/presets", strings.NewReader(
		`{"base_url":"http://localhost:11434/v1","model":"qwen3:8b"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	server.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created api.LLMPreset
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created preset: %v", err)
	}

	// Then the user preset can be deleted while the built-in preset remains protected.
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/llm/presets/"+strconv.FormatInt(created.ID, 10), nil)
	deleteRecorder := httptest.NewRecorder()
	server.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	protectedRequest := httptest.NewRequest(http.MethodDelete, "/api/llm/presets/"+strconv.FormatInt(bootstrap.LLMPresets[0].ID, 10), nil)
	protectedRecorder := httptest.NewRecorder()
	server.ServeHTTP(protectedRecorder, protectedRequest)
	if protectedRecorder.Code != http.StatusForbidden {
		t.Fatalf("built-in delete status = %d, want %d", protectedRecorder.Code, http.StatusForbidden)
	}
}

func newTestServer(t *testing.T, fetcher feed.Fetcher) (*web.Server, *feed.Repository, *settings.Repository) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "dashboard.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	settingsRepository, err := settings.NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open settings: %v", err)
	}
	feedRepository := feed.NewRepository(db)
	reportRepository := report.NewRepository(db)
	summaryService := summary.NewService(settingsRepository)
	assets := fs.FS(fstest.MapFS{"index.html": {Data: []byte("ok")}})
	server := web.NewServer(web.Dependencies{
		Assets: assets, Feeds: feedRepository,
		Collector: feed.NewCollector(feedRepository, fetcher),
		Dashboard: dashboard.NewRepository(db), Settings: settingsRepository,
		Reports: reportRepository, ReportService: report.NewService(reportRepository, summaryService),
		Summaries: summaryService,
	})
	return server, feedRepository, settingsRepository
}

func compatibleLLM(t *testing.T, value string) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		content, err := json.Marshal(map[string]string{"summary": value})
		if err != nil {
			t.Errorf("encode completion content: %v", err)
		}
		response := map[string]any{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1, "model": "test-model",
			"choices": []map[string]any{{
				"index": 0, "message": map[string]string{"role": "assistant", "content": string(content)},
				"finish_reason": "stop",
			}},
		}
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode completion response: %v", err)
		}
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

func configureLLM(t *testing.T, repository *settings.Repository, baseURL string) {
	t.Helper()
	value, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	value.LLMBaseURL = baseURL + "/v1"
	value.LLMModel = "test-model"
	value.LLMAPIKey = "test-key"
	if err := repository.Save(context.Background(), value); err != nil {
		t.Fatalf("save settings: %v", err)
	}
}

type stubFetcher struct {
	document feed.Document
}

func (f *stubFetcher) Fetch(context.Context, api.Source) (feed.Document, error) {
	return f.document, nil
}
