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

// TrendPoint is one bucket of the rolling window in Dashboard responses from GET /api/dashboard.
type TrendPoint struct {
	Start string `json:"start"`
	End   string `json:"end"`
	// Collection volume; each severity counts only its own band.
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	// Attribution quality; the three fields below partition Attributed by precision.
	Attributed       int `json:"attributed"`
	UnknownActor     int `json:"unknown_actor"`
	QualifiedUnknown int `json:"qualified_unknown"`
	NamedActor       int `json:"named_actor"`
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
	Trend         []TrendPoint   `json:"trend"`
	CVEs          []CVEInsight   `json:"cves"`
}
