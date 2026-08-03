package web

import (
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestParseCollectableDayEnforcesRetentionWindow(t *testing.T) {
	// Given a fixed local day and retention boundary cases.
	now := time.Date(2026, 8, 3, 23, 30, 0, 0, time.FixedZone("KST", 9*60*60))
	tests := []struct {
		name    string
		day     string
		wantErr bool
	}{
		{name: "today", day: "2026-08-03"},
		{name: "oldest included", day: "2026-07-25"},
		{name: "expired", day: "2026-07-24", wantErr: true},
		{name: "future", day: "2026-08-04", wantErr: true},
		{name: "malformed", day: "2026-8-3", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When the date is parsed at the collection boundary.
			_, err := parseCollectableDay(test.day, now)

			// Then only today through today minus nine days are accepted.
			if (err != nil) != test.wantErr {
				t.Fatalf("parseCollectableDay(%q) error = %v, wantErr %v", test.day, err, test.wantErr)
			}
		})
	}
}

func TestValidateSettingsRejectsInvalidWireValues(t *testing.T) {
	// Given a valid public settings request and one invalid field per case.
	valid := api.Settings{Language: "ko", Theme: "dark", LLMBaseURL: "https://api.openai.com/v1", LLMModel: "gpt-4o-mini", LLMTimeout: 60}
	tests := []struct {
		name   string
		mutate func(*api.Settings)
	}{
		{name: "language", mutate: func(value *api.Settings) { value.Language = "jp" }},
		{name: "theme", mutate: func(value *api.Settings) { value.Theme = "system" }},
		{name: "relative URL", mutate: func(value *api.Settings) { value.LLMBaseURL = "/v1" }},
		{name: "FTP URL", mutate: func(value *api.Settings) { value.LLMBaseURL = "ftp://example.com/v1" }},
		{name: "model", mutate: func(value *api.Settings) { value.LLMModel = " " }},
		{name: "timeout lower bound", mutate: func(value *api.Settings) { value.LLMTimeout = 0 }},
		{name: "timeout upper bound", mutate: func(value *api.Settings) { value.LLMTimeout = 601 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid
			test.mutate(&value)

			// When the HTTP boundary validates the request.
			err := validateSettings(value)

			// Then the invalid field is rejected.
			if err == nil {
				t.Fatalf("validateSettings(%+v) returned nil", value)
			}
		})
	}
}
