package api

// Settings is the request body for PUT /api/settings and POST /api/llm/test.
type Settings struct {
	Language              string `json:"language"`
	Accent                string `json:"accent"`
	LLMBaseURL            string `json:"llm_base_url"`
	LLMModel              string `json:"llm_model"`
	LLMAPIKey             string `json:"llm_api_key,omitempty"`
	LLMTimeout            int    `json:"llm_timeout"`
	NVDAPIKey             string `json:"nvd_api_key,omitempty"`
	TimezoneOffsetMinutes int    `json:"timezone_offset_minutes"`
}

// SaveSettingsRequest is the request body for PUT /api/settings. An absent
// sources list leaves every source enabled flag untouched.
type SaveSettingsRequest struct {
	Settings
	Sources []SourceState `json:"sources,omitempty"`
}

// SettingsResponse is returned by GET /api/bootstrap and PUT /api/settings.
// Secret values are represented only by configured flags and never serialized.
// Theme is deliberately absent: it is browser-local, kept only in localStorage.
type SettingsResponse struct {
	Language              string `json:"language"`
	Accent                string `json:"accent"`
	LLMBaseURL            string `json:"llm_base_url"`
	LLMModel              string `json:"llm_model"`
	LLMAPIKeyConfigured   bool   `json:"llm_api_key_configured"`
	LLMTimeout            int    `json:"llm_timeout"`
	NVDAPIKeyConfigured   bool   `json:"nvd_api_key_configured"`
	TimezoneOffsetMinutes int    `json:"timezone_offset_minutes"`
}

// UpdateLanguageRequest is the request body for PATCH /api/settings/language.
type UpdateLanguageRequest struct {
	Language string `json:"language"`
}

// LLMPreset is the stored form used by the settings feature.
type LLMPreset struct {
	ID      int64  `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"-"`
	Builtin bool   `json:"builtin"`
}

// LLMPresetResponse is returned by GET /api/bootstrap, GET /api/llm/presets,
// and POST /api/llm/presets. It never contains the stored API key.
type LLMPresetResponse struct {
	ID               int64  `json:"id"`
	Label            string `json:"label"`
	BaseURL          string `json:"base_url"`
	Model            string `json:"model"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	Builtin          bool   `json:"builtin"`
}

// CreateLLMPresetRequest is the request body for POST /api/llm/presets.
type CreateLLMPresetRequest struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key,omitempty"`
}

// UpdateLLMPresetRequest is the request body for PUT /api/llm/presets/:id.
type UpdateLLMPresetRequest struct {
	APIKey string `json:"api_key"`
}
