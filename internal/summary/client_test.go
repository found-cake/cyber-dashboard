package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientGenerate_returnsSummary_whenEndpointSupportsMinimalChatCompletionsContract(t *testing.T) {
	// Given an endpoint that accepts the portable Chat Completions request shape.
	requests := make(chan map[string]any, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requests <- body
		messages, messagesOK := body["messages"].([]any)
		first, firstOK := firstMessage(messages)
		if !messagesOK || !firstOK || first["role"] != "system" || body["response_format"] != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"message":"invalid JSON body","type":"invalid_request_error"}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"summary\":\"검증된 요약\"}"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()
	client, err := NewClient(Config{
		BaseURL: upstream.URL + "/v1",
		Model:   "test-model",
		APIKey:  "test-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// When a summary is generated through the SDK adapter.
	got, err := client.Generate(context.Background(), Request{
		Language: "ko",
		Kind:     "daily",
		Facts:    []string{"Critical 1", "CVE-2026-1547"},
	})

	// Then the structured summary is returned with the portable request shape.
	body := <-requests
	if err != nil {
		t.Fatalf("generate: %v; request = %#v", err, body)
	}
	if got != "검증된 요약" {
		t.Fatalf("summary = %q, want 검증된 요약", got)
	}
	if body["model"] != "test-model" {
		t.Fatalf("model = %v, want test-model", body["model"])
	}
	if _, exists := body["max_tokens"]; exists {
		t.Fatalf("max_tokens must be omitted: %#v", body)
	}
	if _, exists := body["max_completion_tokens"]; exists {
		t.Fatalf("max_completion_tokens must be omitted: %#v", body)
	}
}

func TestClientAnalyzeArticleSendsFullBody_andReturnsImpactSignals(t *testing.T) {
	// Given an OpenAI-compatible endpoint and a body marker that appears only at the article end.
	requests := make(chan map[string]any, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requests <- body
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"summary\":\"Incident summary\",\"attack_method\":\"Supply chain\",\"threat_actor\":\"Unknown\",\"actor_country\":\"\",\"target_sector\":\"IT\",\"victim_count\":15000,\"zero_day\":true}"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()
	client, err := NewClient(Config{BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "key", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	fullBody := "Opening paragraph.\n\nTechnical details.\n\nFULL_BODY_END_MARKER"

	// When the article is analyzed.
	analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{
		Language: "en", Title: "Supply-chain incident", URL: "https://example.com/story", Body: fullBody,
	})

	// Then the complete body reaches the endpoint and structured impact signals are parsed.
	requestBody := <-requests
	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	messages, ok := requestBody["messages"].([]any)
	if !ok || len(messages) < 2 {
		t.Fatalf("messages = %#v", requestBody["messages"])
	}
	userMessage, ok := messages[1].(map[string]any)
	content, _ := userMessage["content"].(string)
	if !ok || !strings.Contains(content, "FULL_BODY_END_MARKER") {
		t.Fatalf("user message does not contain the complete body: %#v", userMessage)
	}
	if analysis.VictimCount != 15000 || !analysis.ZeroDay || analysis.Summary != "Incident summary" {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestClientAnalyzeArticleAcceptsJSONCodeFence(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		content := "```json\n{\"summary\":\"Incident summary\",\"attack_method\":\"Malware\",\"threat_actor\":\"Unknown\",\"actor_country\":\"\",\"target_sector\":\"Finance\",\"victim_count\":0,\"zero_day\":false}\n```"
		encodedContent, _ := json.Marshal(content)
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encodedContent)
	}))
	defer upstream.Close()
	client, err := NewClient(Config{BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "key", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "en", Title: "Incident", Body: "Body"})

	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	if analysis.Summary != "Incident summary" || analysis.TargetSector != "Finance" {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestClientAnalyzeArticleTreatsNumericZeroCountryAsUnknown(t *testing.T) {
	// Given a compatible endpoint that uses numeric zero for an unknown actor country.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"summary\":\"Security strategy\",\"attack_method\":\"AI-driven attacks\",\"threat_actor\":\"attackers\",\"actor_country\":0,\"target_sector\":\"enterprise security\",\"victim_count\":500,\"zero_day\":false}"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()
	client, err := NewClient(Config{BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "key", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// When the article response is analyzed.
	analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "en", Title: "AI security", Body: "Body"})

	// Then the usable analysis is retained and zero becomes an empty optional country.
	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	if analysis.ActorCountry != "" || analysis.Summary != "Security strategy" {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestClientAnalyzeArticleUsesUnknownActor_whenResponseLeavesThreatActorEmpty(t *testing.T) {
	// Given an otherwise complete analysis whose article does not name a threat actor.
	client := newArticleAnalysisClient(t, `{
		"summary":"Adobe fixed critical vulnerabilities.",
		"attack_method":"SQL injection",
		"threat_actor":"",
		"actor_country":"",
		"target_sector":"Enterprise software",
		"victim_count":0,
		"zero_day":false
	}`)

	// When the compatible endpoint response is parsed.
	analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "ko", Title: "Adobe update", Body: "Body"})

	// Then the unnamed actor is represented as unknown instead of rejecting the whole analysis.
	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	if analysis.ThreatActor != "Unknown" {
		t.Fatalf("threat actor = %q, want Unknown", analysis.ThreatActor)
	}
}

func TestClientAnalyzeArticleJoinsAttackMethods_whenResponseUsesArray(t *testing.T) {
	// Given a compatible endpoint that returns multiple attack methods as a JSON array.
	client := newArticleAnalysisClient(t, `{
		"summary":"DNS security products address several attack techniques.",
		"attack_method":["피싱","악성코드 콜백","DNS 터널링"],
		"threat_actor":"",
		"actor_country":"",
		"target_sector":"기업",
		"victim_count":0,
		"zero_day":false
	}`)

	// When the compatible endpoint response is parsed.
	analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "ko", Title: "DNS security", Body: "Body"})

	// Then the list is retained as one display-safe category value.
	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	if analysis.AttackMethod != "피싱, 악성코드 콜백, DNS 터널링" {
		t.Fatalf("attack method = %q", analysis.AttackMethod)
	}
}

func TestClientGenerateAcceptsConciseSummaryField(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"concise_summary\":\"Compatible summary\"}"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()
	client, err := NewClient(Config{BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "key", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	value, err := client.Generate(context.Background(), Request{Language: "en", Kind: "daily", Facts: []string{"article"}})

	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if value != "Compatible summary" {
		t.Fatalf("summary = %q, want Compatible summary", value)
	}
}

func firstMessage(messages []any) (map[string]any, bool) {
	if len(messages) == 0 {
		return nil, false
	}
	value, ok := messages[0].(map[string]any)
	return value, ok
}

func newArticleAnalysisClient(t *testing.T, content string) *Client {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		encodedContent, err := json.Marshal(content)
		if err != nil {
			t.Errorf("encode content: %v", err)
			return
		}
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encodedContent)
	}))
	t.Cleanup(upstream.Close)
	client, err := NewClient(Config{BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "key", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}
