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

func TestClientGenerate_returnsEnglishSummary_whenLanguageIsEnglish(t *testing.T) {
	// Given an endpoint that follows the explicit output-language contract.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		summary := "한글 요약"
		if len(body.Messages) > 0 && strings.Contains(body.Messages[0].Content, `<output_language code="en">`) {
			summary = "English summary"
		}
		content, err := json.Marshal(map[string]string{"summary": summary})
		if err != nil {
			t.Errorf("encode content: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}]}`, content)
	}))
	defer upstream.Close()
	client, err := NewClient(Config{BaseURL: upstream.URL + "/v1", Model: "test-model", APIKey: "key", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// When an English daily summary is generated from Korean source facts.
	value, err := client.Generate(context.Background(), Request{
		Language: "en", Kind: "daily", Facts: []string{"한국어로 작성된 보안 기사"},
	})

	// Then the endpoint returns an English summary.
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if value != "English summary" {
		t.Fatalf("summary = %q, want English summary", value)
	}
}
