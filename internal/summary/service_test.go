package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

type staticSettings struct {
	value api.Settings
}

func (s staticSettings) Get(context.Context) (api.Settings, error) {
	return s.value, nil
}

// summaryCall records one captured upstream request: exactly one of its fields is populated,
// because a batch request carries facts and the merge request carries the section summaries.
type summaryCall struct {
	Facts    []string `json:"facts"`
	Sections []string `json:"sections"`
}

// recordSummaryCalls serves an OpenAI-compatible endpoint that records every request and
// answers each one with reply(callNumber).
func recordSummaryCalls(t *testing.T, calls *[]summaryCall, reply func(int) string) *Service {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode chat request: %v", err)
		}
		var input summaryCall
		if len(body.Messages) < 2 {
			t.Errorf("messages = %d, want at least 2", len(body.Messages))
		} else if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
			t.Errorf("decode summary input: %v", err)
		}
		*calls = append(*calls, input)
		encodedContent, _ := json.Marshal(reply(len(*calls)))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encodedContent)
	}))
	t.Cleanup(upstream.Close)
	return NewService(staticSettings{value: api.Settings{
		LLMBaseURL: upstream.URL + "/v1", LLMModel: "test-model", LLMAPIKey: "key", LLMTimeout: 5,
	}})
}

func articleFacts(count int) []string {
	facts := make([]string, count)
	for index := range facts {
		facts[index] = fmt.Sprintf("article-%d", index+1)
	}
	return facts
}

func TestServiceGenerateJoinsBatches_whenDailyFitsInTwoParagraphs(t *testing.T) {
	// Given ten article facts, which split into the two batches that stay readable when joined.
	calls := []summaryCall{}
	service := recordSummaryCalls(t, &calls, func(call int) string {
		return fmt.Sprintf(`{"summary":"part-%d"}`, call)
	})

	// When the daily summary is generated.
	got, err := service.Generate(context.Background(), Request{Language: "ko", Kind: "daily", Facts: articleFacts(10)})

	// Then each batch stays its own paragraph and no merge request is made.
	if err != nil {
		t.Fatalf("generate summary: %v", err)
	}
	if got != "part-1\n\npart-2" {
		t.Fatalf("summary = %q, want one paragraph per batch", got)
	}
	if len(calls) != 2 {
		t.Fatalf("request count = %d, want 2 batch requests", len(calls))
	}
}

func TestServiceGenerateMergesBatches_whenDailyExceedsTwoParagraphs(t *testing.T) {
	// Given fourteen article facts, which split into more batches than stay readable when joined.
	calls := []summaryCall{}
	service := recordSummaryCalls(t, &calls, func(call int) string {
		if call == 4 {
			return `{"summary":"overview\n\n■ topic\n- item"}`
		}
		return fmt.Sprintf(`{"summary":"part-%d"}`, call)
	})

	// When the daily summary is generated.
	got, err := service.Generate(context.Background(), Request{Language: "ko", Kind: "daily", Facts: articleFacts(14)})

	// Then a final pass merges the three batches into one grouped summary.
	if err != nil {
		t.Fatalf("generate summary: %v", err)
	}
	if got != "overview\n\n■ topic\n- item" {
		t.Fatalf("summary = %q, want the merged summary", got)
	}
	if len(calls) != 4 {
		t.Fatalf("request count = %d, want 3 batch requests and 1 merge request", len(calls))
	}
	// And every batch request stays within the fact limit while the merge sees all three parts.
	for index, call := range calls[:3] {
		if len(call.Facts) > 5 {
			t.Fatalf("request %d facts = %d, want at most 5", index+1, len(call.Facts))
		}
	}
	if want := []string{"part-1", "part-2", "part-3"}; !slices.Equal(calls[3].Sections, want) {
		t.Fatalf("merge sections = %q, want %q", calls[3].Sections, want)
	}
}

func TestServiceGenerateFallsBackToBatches_whenMergeFails(t *testing.T) {
	// Given a merge pass that never returns usable JSON.
	calls := []summaryCall{}
	service := recordSummaryCalls(t, &calls, func(call int) string {
		if call > 3 {
			return "not JSON"
		}
		return fmt.Sprintf(`{"summary":"part-%d"}`, call)
	})

	// When the daily summary is generated.
	got, err := service.Generate(context.Background(), Request{Language: "ko", Kind: "daily", Facts: articleFacts(14)})

	// Then the batches are served instead of losing a summary that already holds every fact.
	if err != nil {
		t.Fatalf("generate summary: %v", err)
	}
	if got != "part-1\n\npart-2\n\npart-3" {
		t.Fatalf("summary = %q, want the joined batches", got)
	}
	if len(calls) != 5 {
		t.Fatalf("request count = %d, want 3 batch requests and 2 merge attempts", len(calls))
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
