package summary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientGenerate_returnsSummary_whenEndpointIsOpenAICompatible(t *testing.T) {
	// Given an OpenAI Chat Completions-compatible endpoint.
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

	// Then the structured summary is returned and the configured model is sent.
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got != "검증된 요약" {
		t.Fatalf("summary = %q, want 검증된 요약", got)
	}
	body := <-requests
	if body["model"] != "test-model" {
		t.Fatalf("model = %v, want test-model", body["model"])
	}
}
