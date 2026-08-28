package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
