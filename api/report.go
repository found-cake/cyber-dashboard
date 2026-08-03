package api

// Report is returned by GET /api/bootstrap, GET /api/reports, and POST /api/reports.
type Report struct {
	ID          int64    `json:"id"`
	Type        string   `json:"type"`
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
	Total       int      `json:"total"`
	Critical    int      `json:"critical"`
	High        int      `json:"high"`
	Medium      int      `json:"medium"`
	TopThreat   string   `json:"top_threat"`
	Actors      []string `json:"actors"`
	Sectors     []string `json:"sectors"`
	Summary     string   `json:"summary"`
	GeneratedAt string   `json:"generated_at"`
}

// CreateReportRequest is the request body for POST /api/reports.
type CreateReportRequest struct {
	Type  string `json:"type"`
	Start string `json:"period_start"`
	End   string `json:"period_end"`
}
