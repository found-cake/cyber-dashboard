package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed"
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
		completed := collectAndWait(t, server, body)
		if completed.Status != api.CollectionCompleted {
			t.Fatalf("job = %+v, want completed", completed)
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
	completed := collectAndWait(t, server, `{"date":"2026-08-03"}`)

	// Then the SDK result is persisted as the daily summary.
	if completed.Status != api.CollectionCompleted {
		t.Fatalf("job = %+v, want completed", completed)
	}
	daily, err := feeds.Daily(context.Background(), "2026-08-03")
	if err != nil {
		t.Fatalf("load daily: %v", err)
	}
	if daily.Summary != "일간 보안 요약" {
		t.Fatalf("summary = %q, want 일간 보안 요약", daily.Summary)
	}
}

func TestUpdateLanguagePersistsEnglish_whenUserSelectsEnglish(t *testing.T) {
	// Given a new server whose persisted language is Korean.
	server, _, appSettings := newTestServer(t, &stubFetcher{})

	// When the user selects English through the language preference API.
	request := httptest.NewRequest(http.MethodPatch, "/api/settings/language", strings.NewReader(`{"language":"en"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then English is persisted for background summary generation.
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	value, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if value.Language != "en" {
		t.Fatalf("language = %q, want en", value.Language)
	}
}

func TestSaveSettingsAppliesSourceStates_whenTheDraftCarriesThem(t *testing.T) {
	// Given a server whose second source is enabled.
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	draft, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMTimeout = 90
	body, err := json.Marshal(api.SaveSettingsRequest{Settings: draft, Sources: []api.SourceState{{ID: 2, Enabled: false}}})
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the settings are saved with the source change attached.
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the one request persists both the settings and the source state.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	sources, err := feeds.Sources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) < 2 || sources[1].Enabled {
		t.Fatalf("sources = %+v, want the second source disabled", sources)
	}
	saved, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMTimeout != 90 {
		t.Fatalf("timeout = %d, want 90", saved.LLMTimeout)
	}
}

func TestSaveSettingsKeepsEverything_whenASourceStateIsUnknown(t *testing.T) {
	// Given a draft that changes a valid setting alongside a source that does not exist.
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	draft, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMTimeout = 90
	body, err := json.Marshal(api.SaveSettingsRequest{
		Settings: draft, Sources: []api.SourceState{{ID: 2, Enabled: false}, {ID: 999, Enabled: false}},
	})
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the save is submitted.
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then neither the sources nor the settings are half applied.
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	sources, err := feeds.Sources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) < 2 || !sources[1].Enabled {
		t.Fatalf("sources = %+v, want the second source still enabled", sources)
	}
	saved, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMTimeout == 90 {
		t.Fatal("settings were saved even though a source state was rejected")
	}
}

func TestCreateReportGeneratesSummary_whenLLMIsConfigured(t *testing.T) {
	// Given collected article data and a configured compatible LLM.
	upstream := compatibleLLM(t, "주간 보안 요약")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	configured, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	configured.TimezoneOffsetMinutes = 9 * 60
	if err := appSettings.Save(context.Background(), configured); err != nil {
		t.Fatalf("save timezone: %v", err)
	}
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
	if !strings.HasSuffix(got.GeneratedAt, "+09:00") {
		t.Fatalf("generated_at = %q, want configured +09:00 offset", got.GeneratedAt)
	}
}

func TestDeleteReportRemovesStoredReport_whenIDExists(t *testing.T) {
	// Given a report created through the public API.
	upstream := compatibleLLM(t, "Disposable report summary")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, feed.FeedArticle{
		ID: "sha256:delete-report", URL: "https://example.com/delete-report", Title: "Disposable threat",
		Description: "Disposable report detail",
	}, "2026-08-02"); err != nil {
		t.Fatalf("save article: %v", err)
	}
	createRequest := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(
		`{"type":"weekly","period_start":"2026-08-01","period_end":"2026-08-03"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	server.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created api.Report
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created report: %v", err)
	}

	// When the report is deleted through its public route.
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/reports/"+strconv.FormatInt(created.ID, 10), nil)
	deleteRecorder := httptest.NewRecorder()
	server.ServeHTTP(deleteRecorder, deleteRequest)

	// Then the API acknowledges deletion and excludes it from subsequent lists.
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/reports", nil)
	listRecorder := httptest.NewRecorder()
	server.ServeHTTP(listRecorder, listRequest)
	var listed []api.Report
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode report list: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed reports = %d, want 0", len(listed))
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

func TestLLMConnectionTestUsesDraftSettings_withoutPersistingThem(t *testing.T) {
	// Given a compatible draft endpoint that differs from persisted settings.
	upstream := compatibleLLM(t, "unused")
	server, _, appSettings := newTestServer(t, &stubFetcher{})
	draft, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	persistedModel := draft.LLMModel
	draft.LLMBaseURL = upstream.URL + "/v1"
	draft.LLMModel = "draft-model"
	draft.LLMAPIKey = "draft-key"
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encode draft settings: %v", err)
	}

	// When the connection is tested with the draft request body.
	request := httptest.NewRequest(http.MethodPost, "/api/llm/test", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the draft succeeds but persisted settings stay unchanged.
	if recorder.Code != http.StatusOK {
		t.Fatalf("test status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	loaded, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if loaded.LLMModel != persistedModel {
		t.Fatalf("persisted model = %q, want %q", loaded.LLMModel, persistedModel)
	}
}
