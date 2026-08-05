package summary

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClientGenerateAcceptsInlineJSONCodeFence(t *testing.T) {
	// Given valid JSON wrapped in a code fence without a newline after the language marker.
	client := newArticleAnalysisClient(t, "```json{\"summary\":\"Compact summary\"}```")

	// When the summary response is parsed.
	summary, err := client.Generate(context.Background(), Request{Language: "en", Kind: "daily", Facts: []string{"fact"}})

	// Then only the fence is removed and the valid JSON is accepted.
	if err != nil {
		t.Fatalf("generate summary: %v", err)
	}
	if summary != "Compact summary" {
		t.Fatalf("summary = %q, want Compact summary", summary)
	}
}

func TestClientGenerateRejectsInlineJSONCodeFenceWhenPayloadIsInvalidJSON(t *testing.T) {
	// Given malformed JSON wrapped in the same compact code-fence format.
	client := newArticleAnalysisClient(t, "```json{\"summary\":}```")

	// When the summary response is parsed.
	_, err := client.Generate(context.Background(), Request{Language: "en", Kind: "daily", Facts: []string{"fact"}})

	// Then strict JSON validation still rejects the malformed payload.
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("error = %v, want ErrInvalidResponse", err)
	}
}

func TestClientAnalyzeArticleAcceptsJSONCodeFenceWithLineComments(t *testing.T) {
	// Given a fenced JSON response containing model-generated line comments and a URL string.
	content := strings.Join([]string{
		"```json",
		"{",
		`  "summary":"Incident details: https://example.com/report",`,
		`  "attack_method":"AI-assisted intrusion",`,
		`  "threat_actor":"Unknown",`,
		`  "actor_country":"Unknown",`,
		`  "target_sector":"IT",`,
		`  "victim_count":3, // affected companies`,
		`  "zero_day":true // exploited zero-day`,
		"}",
		"```",
	}, "\n")
	client := newArticleAnalysisClient(t, content)

	// When the compatible endpoint response is analyzed.
	analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "en", Title: "Incident", Body: "Body"})

	// Then comments are ignored without corrupting slashes inside JSON strings.
	if err != nil {
		t.Fatalf("analyze article: %v", err)
	}
	if analysis.Summary != "Incident details: https://example.com/report" || analysis.VictimCount != 3 || !analysis.ZeroDay {
		t.Fatalf("analysis = %+v", analysis)
	}
}
