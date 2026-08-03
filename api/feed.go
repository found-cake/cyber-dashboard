package api

// Source appears in Bootstrap responses from GET /api/bootstrap.
type Source struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Slug    string `json:"slug"`
	Enabled bool   `json:"enabled"`
}

// Article appears in Daily responses from GET /api/daily/:day.
type Article struct {
	ID           int64    `json:"id"`
	Source       string   `json:"source"`
	FeedUID      string   `json:"feed_uid"`
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	PublishedAt  string   `json:"published_at"`
	Summary      string   `json:"summary"`
	AttackMethod string   `json:"attack_method"`
	ThreatActor  string   `json:"threat_actor"`
	ActorCountry string   `json:"actor_country,omitempty"`
	Sector       string   `json:"sector"`
	Severity     string   `json:"severity"`
	CVEs         []string `json:"cves"`
}

// Daily is returned by GET /api/daily/:day.
type Daily struct {
	Day      string    `json:"day"`
	Summary  string    `json:"summary"`
	Articles []Article `json:"articles"`
}

// CollectionResult is returned by POST /api/collect.
type CollectionResult struct {
	Day       string   `json:"day"`
	Collected int      `json:"collected"`
	Sources   int      `json:"sources"`
	Warnings  []string `json:"warnings"`
}

// CollectRequest is the request body for POST /api/collect.
type CollectRequest struct {
	Day string `json:"date"`
}

// ToggleSourceRequest is the request body for PATCH /api/sources/:id.
type ToggleSourceRequest struct {
	Enabled bool `json:"enabled"`
}
