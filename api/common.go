package api

// Bootstrap is returned by GET /api/bootstrap.
type Bootstrap struct {
	Sources       []Source       `json:"sources"`
	Reports       []Report       `json:"reports"`
	Settings      Settings       `json:"settings"`
	LLMPresets    []LLMPreset    `json:"llm_presets"`
	CollectedDays []string       `json:"collected_days"`
	Collection    *CollectionJob `json:"collection,omitempty"`
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
