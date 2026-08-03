package api

// BreakdownRow appears in Dashboard responses from GET /api/dashboard.
type BreakdownRow struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// CVEInsight appears in Dashboard responses from GET /api/dashboard.
type CVEInsight struct {
	ID              string  `json:"id"`
	CVSS            float64 `json:"cvss"`
	AffectedProduct string  `json:"affected_product"`
	FirstSeen       string  `json:"first_seen"`
	Mentions        int     `json:"mentions"`
}

// Dashboard is returned by GET /api/dashboard.
type Dashboard struct {
	Empty         bool           `json:"empty"`
	Total         int            `json:"total"`
	Critical      int            `json:"critical"`
	High          int            `json:"high"`
	CVECount      int            `json:"cve_count"`
	AttackMethods []BreakdownRow `json:"attack_methods"`
	ThreatActors  []BreakdownRow `json:"threat_actors"`
	CVEs          []CVEInsight   `json:"cves"`
}
