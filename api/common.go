package api

// Bootstrap is returned by GET /api/bootstrap.
type Bootstrap struct {
	Auth          AuthState           `json:"auth"`
	Sources       []Source            `json:"sources"`
	Reports       []Report            `json:"reports"`
	Settings      SettingsResponse    `json:"settings"`
	LLMPresets    []LLMPresetResponse `json:"llm_presets"`
	CollectedDays []string            `json:"collected_days"`
	Collection    *CollectionJob      `json:"collection,omitempty"`
	CVERefresh    *CVERefreshJob      `json:"cve_refresh,omitempty"`
}

type AuthState struct {
	Enabled       bool `json:"enabled"`
	Authenticated bool `json:"authenticated"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// HealthResponse is returned by GET /healthz.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorResponse is returned by API endpoints for HTTP error responses.
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	MessageKO string `json:"message_ko"`
	MessageEN string `json:"message_en"`
}

// LLMTestResponse is returned by POST /api/llm/test.
type LLMTestResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	MessageKO string `json:"message_ko,omitempty"`
	MessageEN string `json:"message_en,omitempty"`
}
