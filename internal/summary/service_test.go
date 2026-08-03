package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

type staticSettings struct {
	value api.Settings
}

func (s staticSettings) Get(context.Context) (api.Settings, error) {
	return s.value, nil
}

func TestServiceGenerateLimitsEachRequestToFiveFacts_whenDailyHasManyArticles(t *testing.T) {
	// Given fourteen article facts and an OpenAI-compatible endpoint that records every request.
	requests := []Request{}
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode chat request: %v", err)
		}
		var input Request
		if len(body.Messages) < 2 {
			t.Errorf("messages = %d, want at least 2", len(body.Messages))
		} else if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
			t.Errorf("decode summary input: %v", err)
		}
		requests = append(requests, input)
		content := fmt.Sprintf(`{"summary":"part-%d"}`, len(requests))
		encodedContent, _ := json.Marshal(content)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encodedContent)
	}))
	t.Cleanup(upstream.Close)
	service := NewService(staticSettings{value: api.Settings{
		LLMBaseURL: upstream.URL + "/v1", LLMModel: "test-model", LLMAPIKey: "key", LLMTimeout: 5,
	}})
	facts := make([]string, 14)
	for index := range facts {
		facts[index] = fmt.Sprintf("article-%d", index+1)
	}

	// When the daily summary is generated.
	got, err := service.Generate(context.Background(), Request{Language: "ko", Kind: "daily", Facts: facts})

	// Then every LLM request contains at most five facts and each batch becomes one paragraph.
	if err != nil {
		t.Fatalf("generate summary: %v", err)
	}
	if got != "part-1\n\npart-2\n\npart-3" {
		t.Fatalf("summary = %q, want one paragraph per batch", got)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d, want 3 batch requests", len(requests))
	}
	for index, request := range requests {
		if len(request.Facts) > 5 {
			t.Fatalf("request %d facts = %d, want at most 5", index+1, len(request.Facts))
		}
	}
}

func TestServiceAnalyzeArticleRetriesOneInvalidResponse(t *testing.T) {
	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.Header().Set("Content-Type", "application/json")
		content := `not JSON`
		if requestCount == 2 {
			content = `{"summary":"Recovered","attack_method":"Malware","threat_actor":"Unknown","actor_country":"","target_sector":"Finance","victim_count":0,"zero_day":false}`
		}
		encodedContent, _ := json.Marshal(content)
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encodedContent)
	}))
	t.Cleanup(upstream.Close)
	service := NewService(staticSettings{value: api.Settings{
		LLMBaseURL: upstream.URL + "/v1", LLMModel: "test-model", LLMAPIKey: "key", LLMTimeout: 5,
	}})

	analysis, err := service.AnalyzeArticle(context.Background(), ArticleRequest{Language: "en", Title: "Incident", Body: "Body"})

	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if analysis.Summary != "Recovered" {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestServiceGenerateRetriesOneInvalidResponse(t *testing.T) {
	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.Header().Set("Content-Type", "application/json")
		content := `not JSON`
		if requestCount == 2 {
			content = `{"summary":"Recovered"}`
		}
		encodedContent, _ := json.Marshal(content)
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encodedContent)
	}))
	t.Cleanup(upstream.Close)
	service := NewService(staticSettings{value: api.Settings{
		LLMBaseURL: upstream.URL + "/v1", LLMModel: "test-model", LLMAPIKey: "key", LLMTimeout: 5,
	}})

	value, err := service.Generate(context.Background(), Request{Language: "en", Kind: "daily", Facts: []string{"article"}})

	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if value != "Recovered" {
		t.Fatalf("summary = %q, want Recovered", value)
	}
}
