package web

import (
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
)

func settingsResponse(value api.Settings) api.SettingsResponse {
	return api.SettingsResponse{
		Language: value.Language, Accent: value.Accent,
		LLMBaseURL: value.LLMBaseURL, LLMModel: value.LLMModel,
		LLMAPIKeyConfigured: strings.TrimSpace(value.LLMAPIKey) != "", LLMTimeout: value.LLMTimeout,
		NVDAPIKeyConfigured:   strings.TrimSpace(value.NVDAPIKey) != "",
		TimezoneOffsetMinutes: value.TimezoneOffsetMinutes,
	}
}

func llmPresetResponse(value api.LLMPreset) api.LLMPresetResponse {
	return api.LLMPresetResponse{
		ID: value.ID, Label: value.Label, BaseURL: value.BaseURL, Model: value.Model,
		APIKeyConfigured: strings.TrimSpace(value.APIKey) != "", Builtin: value.Builtin,
	}
}

func llmPresetResponses(values []api.LLMPreset) []api.LLMPresetResponse {
	responses := make([]api.LLMPresetResponse, 0, len(values))
	for _, value := range values {
		responses = append(responses, llmPresetResponse(value))
	}
	return responses
}
