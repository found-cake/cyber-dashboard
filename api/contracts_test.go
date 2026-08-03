package api_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestBootstrapMarshalsEmptyCollectionsAsArrays(t *testing.T) {
	// Given a public bootstrap response with initialized empty collections.
	value := api.Bootstrap{
		Sources: []api.Source{}, Reports: []api.Report{}, LLMPresets: []api.LLMPreset{}, CollectedDays: []string{},
	}

	// When an external application serializes the response.
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal bootstrap: %v", err)
	}

	// Then every collection remains an array instead of becoming null.
	for _, field := range []string{"sources", "reports", "llm_presets", "collected_days"} {
		if !strings.Contains(string(encoded), `"`+field+`":[]`) {
			t.Fatalf("%s = %s, want empty array", field, encoded)
		}
	}
}

func TestArticleOmitsEmptyActorCountry(t *testing.T) {
	// Given an article without an attributed country.
	value := api.Article{Title: "Threat", CVEs: []string{}}

	// When it is serialized for an API response.
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal article: %v", err)
	}

	// Then the optional country field is absent from the wire contract.
	if strings.Contains(string(encoded), "actor_country") {
		t.Fatalf("article = %s, actor_country must be omitted", encoded)
	}
}

func TestErrorResponseExposesKoreanAndEnglishMessages_forExternalClients(t *testing.T) {
	// Given a user-facing API error with both supported languages.
	value := api.ErrorResponse{
		Code: "invalid_request", Message: "요청이 올바르지 않습니다 / The request is invalid",
		MessageKO: "요청이 올바르지 않습니다", MessageEN: "The request is invalid",
	}

	// When an external application serializes the response.
	encoded, err := json.Marshal(value)

	// Then both localized fields remain available without parsing the combined message.
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}
	for _, field := range []string{"message_ko", "message_en"} {
		if !strings.Contains(string(encoded), `"`+field+`"`) {
			t.Fatalf("error response = %s, missing %s", encoded, field)
		}
	}
}
