package api

// Settings is returned by GET /api/bootstrap and used by PUT /api/settings.
type Settings struct {
	Language              string `json:"language"`
	Theme                 string `json:"theme"`
	Accent                string `json:"accent"`
	LLMBaseURL            string `json:"llm_base_url"`
	LLMModel              string `json:"llm_model"`
	LLMAPIKey             string `json:"llm_api_key,omitempty"`
	LLMTimeout            int    `json:"llm_timeout"`
	NVDAPIKey             string `json:"nvd_api_key,omitempty"`
	TimezoneOffsetMinutes int    `json:"timezone_offset_minutes"`
}

// LLMPreset is returned by GET /api/bootstrap, GET /api/llm/presets, and POST /api/llm/presets.
type LLMPreset struct {
	ID      int64  `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key,omitempty"`
	Builtin bool   `json:"builtin"`
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
