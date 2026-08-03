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
