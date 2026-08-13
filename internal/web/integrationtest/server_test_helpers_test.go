package integrationtest

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
	"github.com/found-cake/cyber-dashboard/internal/feed/enrichment"
	feedstore "github.com/found-cake/cyber-dashboard/internal/feed/store"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/web"
)

func newTestServer(t *testing.T, fetcher collector.Fetcher) (*web.Server, *feedstore.Repository, *settings.Repository) {
	return newTestServerWithConfig(t, testServerConfig{fetcher: fetcher, nvdAPIKey: "test-nvd-key"})
}

func newTestServerWithNVD(t *testing.T, fetcher collector.Fetcher, nvdAPIKey string) (*web.Server, *feedstore.Repository, *settings.Repository) {
	return newTestServerWithConfig(t, testServerConfig{fetcher: fetcher, nvdAPIKey: nvdAPIKey})
}

func recentCollectionDay() string {
	return time.Now().AddDate(0, 0, -3).Format(time.DateOnly)
}

type testServerConfig struct {
	fetcher         collector.Fetcher
	nvdAPIKey       string
	vulnerabilities web.VulnerabilityEnricher
	now             func() time.Time
}

func newTestServerWithConfig(t *testing.T, config testServerConfig) (*web.Server, *feedstore.Repository, *settings.Repository) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "dashboard.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	settingsRepository, err := settings.NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open settings: %v", err)
	}
	if strings.TrimSpace(config.nvdAPIKey) != "" {
		configureNVD(t, settingsRepository, config.nvdAPIKey)
	}
	feedRepository := feedstore.NewRepository(db)
	reportRepository := report.NewRepository(db)
	summaryService := summary.NewService(settingsRepository)
	assets := fs.FS(fstest.MapFS{"index.html": {Data: []byte("ok")}})
	server := web.NewServer(web.Dependencies{
		Assets: assets, Feeds: feedRepository,
		Collector: collector.NewCollector(feedRepository, config.fetcher, &stubBodyLoader{}),
		Dashboard: dashboard.NewRepository(db), Settings: settingsRepository,
		Reports: reportRepository, ReportService: report.NewService(reportRepository, summaryService),
		Summaries: summaryService, Articles: enrichment.NewArticleEnrichmentService(feedRepository, summaryService),
		Vulnerabilities: config.vulnerabilities,
		Now:             config.now,
		TrustedHosts:    []string{"example.com"},
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
		content, err := json.Marshal(map[string]any{
			"summary": value, "ok": true, "attack_method": "Exploit", "threat_actor": "Unknown",
			"actor_country": "", "target_sector": "IT", "victim_count": 0, "zero_day": false,
		})
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

func configureNVD(t *testing.T, repository *settings.Repository, apiKey string) {
	t.Helper()
	value, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	value.NVDAPIKey = apiKey
	if err := repository.Save(context.Background(), value); err != nil {
		t.Fatalf("save settings: %v", err)
	}
}

func collectAndWait(t *testing.T, server http.Handler, body string) api.CollectionJob {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("collect status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var started api.CollectionJob
	if err := json.Unmarshal(recorder.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode started job: %v", err)
	}
	status := httptest.NewRecorder()
	server.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/collect/"+started.ID+"?wait=1", nil))
	var completed api.CollectionJob
	if err := json.Unmarshal(status.Body.Bytes(), &completed); err != nil {
		t.Fatalf("decode completed job: %v", err)
	}
	return completed
}

type stubFetcher struct {
	document collector.Document
}

type stubBodyLoader struct{}

func (*stubBodyLoader) Load(_ context.Context, _ api.Source, article collector.FeedArticle) (string, error) {
	return article.Description, nil
}

func (f *stubFetcher) Fetch(context.Context, api.Source) (collector.Document, error) {
	return f.document, nil
}
