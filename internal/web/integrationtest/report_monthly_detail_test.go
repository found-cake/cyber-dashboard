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

func TestCreateMonthlyReportUsesDailySummariesAcrossPeriod(t *testing.T) {
	// Given ten active days and a compatible LLM endpoint that records report inputs.
	dailySummaryCount := 0
	articleFactCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode completion request: %v", err)
		}
		var input struct {
			Facts    []string `json:"facts"`
			Sections []string `json:"sections"`
		}
		if len(body.Messages) < 2 {
			t.Errorf("decode report input from %d messages", len(body.Messages))
			return
		}
		if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
			t.Errorf("decode report input: %v", err)
			return
		}
		for _, fact := range input.Facts {
			if strings.HasPrefix(fact, "period=") || strings.HasPrefix(fact, "total=") {
				continue
			}
			if strings.Contains(fact, "Daily digest") {
				dailySummaryCount++
			} else {
				articleFactCount++
			}
		}
		summary := "batch"
		if len(input.Sections) > 0 {
			summary = "Monthly report detail"
		}
		content, err := json.Marshal(map[string]any{"summary": summary})
		if err != nil {
			t.Errorf("encode completion content: %v", err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]any{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1, "model": "test-model",
			"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": string(content)}, "finish_reason": "stop"}},
		}); err != nil {
			t.Errorf("encode completion response: %v", err)
		}
	}))
	t.Cleanup(upstream.Close)
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	for day := 1; day <= 10; day++ {
		date := fmt.Sprintf("2026-08-%02d", day)
		for item := 1; item <= 7; item++ {
			if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
				ID: fmt.Sprintf("monthly-%s-%d", date, item), URL: fmt.Sprintf("https://example.com/%s/%d", date, item),
				Title: fmt.Sprintf("Incident %s item %d", date, item), Description: "Report evidence",
			}, date); err != nil {
				t.Fatalf("save article %s item %d: %v", date, item, err)
			}
		}
		if err := feeds.SaveDailySummary(context.Background(), date, "Daily digest "+date); err != nil {
			t.Fatalf("save daily summary %s: %v", date, err)
		}
	}

	// When a monthly report is created through the HTTP API.
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	response, err := httpServer.Client().Post(httpServer.URL+"/api/reports", "application/json", strings.NewReader(
		`{"type":"monthly","period_start":"2026-08-01","period_end":"2026-08-10"}`))

	// Then the generated report uses all daily summaries and no raw article summary.
	if err != nil {
		t.Fatalf("create monthly report: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	var created api.Report
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode created report: %v", err)
	}
	if created.Summary != "Monthly report detail" || dailySummaryCount != 10 || articleFactCount != 0 {
		t.Fatalf("summary=%q daily summaries=%d article facts=%d", created.Summary, dailySummaryCount, articleFactCount)
	}
}

func TestCreateWeeklyReportRequiresDailySummaryForEveryActiveDay(t *testing.T) {
	// Given one collected article whose day has no generated daily summary.
	upstream := compatibleLLM(t, "unused")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
		ID: "missing-daily-summary", URL: "https://example.com/missing", Title: "Unrepresented incident",
		Description: "This raw article must not become a weekly summary directly.",
	}, "2026-08-01"); err != nil {
		t.Fatalf("save article: %v", err)
	}

	// When a weekly report is requested for that day.
	request := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(
		`{"type":"weekly","period_start":"2026-08-01","period_end":"2026-08-01"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the API reports the missing prerequisite instead of creating a partial report.
	if recorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response api.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Code != "daily_summaries_required" || response.MessageKO == "" || response.MessageEN == "" {
		t.Fatalf("error response = %+v", response)
	}
}
