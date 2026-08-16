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
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

func TestCreateWeeklyReportReturnsTranslatedThreatTitle_whenLanguageIsKorean(t *testing.T) {
	// Given a critical incident and an LLM that localizes the selected threat title.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		content, err := json.Marshal(map[string]any{
			"summary": "주간 보안 요약",
			"top_threat_groups": []map[string]any{{
				"representative_id": "threat-1", "member_ids": []string{"threat-1"},
				"translated_title": "대학 개인정보 18만 건 유출",
			}},
		})
		if err != nil {
			t.Errorf("encode completion content: %v", err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]any{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1, "model": "test-model",
			"choices": []map[string]any{{
				"index": 0, "message": map[string]string{"role": "assistant", "content": string(content)},
				"finish_reason": "stop",
			}},
		}); err != nil {
			t.Errorf("encode completion response: %v", err)
		}
	}))
	t.Cleanup(upstream.Close)
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	configured, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	configured.Language = "ko"
	if err := appSettings.Save(context.Background(), configured); err != nil {
		t.Fatalf("save language: %v", err)
	}
	if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
		ID: "translated-report-threat", URL: "https://example.com/breach", Title: "University leaks 180,000 personal records",
		Description: "Large university data breach", Body: "A university disclosed a breach affecting 180,000 people.",
	}, "2026-08-15"); err != nil {
		t.Fatalf("save article: %v", err)
	}
	articles, err := feeds.ArticlesForAnalysis(context.Background(), "2026-08-15")
	if err != nil || len(articles) != 1 {
		t.Fatalf("articles for analysis = %+v, error = %v", articles, err)
	}
	if err := feeds.SaveArticleAnalysis(context.Background(), articles[0].ID, summary.ArticleAnalysis{
		Summary: "Large university data breach", AttackMethod: "Data Breach / Unauthorized Access",
		ThreatActor: "Unknown", TargetSector: "Education / Research", VictimCount: 180000,
	}); err != nil {
		t.Fatalf("save article analysis: %v", err)
	}
	if err := feeds.SaveDailySummary(context.Background(), "2026-08-15", "대학 개인정보 유출 사고가 확인됐다."); err != nil {
		t.Fatalf("save daily summary: %v", err)
	}

	// When a weekly report is created through the HTTP API.
	request := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(
		`{"type":"weekly","period_start":"2026-08-09","period_end":"2026-08-15"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the localized title is returned as the report's persisted threat title.
	if recorder.Code != http.StatusCreated {
		t.Fatalf("report status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var created api.Report
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if len(created.TopThreats) != 1 || created.TopThreats[0].Title != "대학 개인정보 18만 건 유출" ||
		created.TopThreat != created.TopThreats[0].Title {
		t.Fatalf("created top threats = %+v, legacy title = %q", created.TopThreats, created.TopThreat)
	}
}
