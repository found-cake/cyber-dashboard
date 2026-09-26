package api

// ReportSummary is the sidebar entry in Bootstrap; GET /api/reports/{id} returns the body.
type ReportSummary struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

// Report is returned by GET /api/reports, GET /api/reports/{id}, and POST /api/reports.
type Report struct {
	ID          int64          `json:"id"`
	Type        string         `json:"type"`
	PeriodStart string         `json:"period_start"`
	PeriodEnd   string         `json:"period_end"`
	Total       int            `json:"total"`
	Critical    int            `json:"critical"`
	High        int            `json:"high"`
	Medium      int            `json:"medium"`
	TopThreat   string         `json:"top_threat"`
	TopThreats  []ReportThreat `json:"top_threats"`
	Actors      []string       `json:"actors"`
	Sectors     []string       `json:"sectors"`
	Summary     string         `json:"summary"`
	GeneratedAt string         `json:"generated_at"`
	Charts      *ReportCharts  `json:"charts,omitempty"`
}

type ReportCharts struct {
	AttackMethods []BreakdownRow       `json:"attack_methods"`
	ThreatActors  []BreakdownRow       `json:"threat_actors"`
	Previous      *ReportChartBaseline `json:"previous,omitempty"`
}

type ReportChartBaseline struct {
	ReportID      int64          `json:"report_id"`
	PeriodStart   string         `json:"period_start"`
	PeriodEnd     string         `json:"period_end"`
	AttackMethods []BreakdownRow `json:"attack_methods"`
	ThreatActors  []BreakdownRow `json:"threat_actors"`
}

type ReportThreat struct {
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	PublishedAt string `json:"published_at"`
	SourceCount int    `json:"source_count"`
}

// CreateReportRequest is the request body for POST /api/reports.
type CreateReportRequest struct {
	Type  string `json:"type"`
	Start string `json:"period_start"`
	End   string `json:"period_end"`
}
