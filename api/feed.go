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
	Body         string   `json:"-"`
	Summary      string   `json:"summary"`
	AttackMethod string   `json:"attack_method"`
	ThreatActor  string   `json:"threat_actor"`
	ActorCountry string   `json:"actor_country,omitempty"`
	Sector       string   `json:"sector"`
	VictimCount  int      `json:"victim_count"`
	ZeroDay      bool     `json:"zero_day"`
	Severity     string   `json:"severity"`
	CVEs         []string `json:"cves"`
}

// Daily is returned by GET /api/daily/:day.
type Daily struct {
	Day      string    `json:"day"`
	Summary  string    `json:"summary"`
	Articles []Article `json:"articles"`
}

// CollectionResult is included in a completed collection job response.
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

type CollectionStatus string

const (
	CollectionRunning   CollectionStatus = "running"
	CollectionCompleted CollectionStatus = "completed"
	CollectionFailed    CollectionStatus = "failed"
	CollectionCancelled CollectionStatus = "cancelled"
)

// CollectionJob is returned by POST and GET /api/collect endpoints.
type CollectionJob struct {
	ID     string            `json:"id"`
	Day    string            `json:"day"`
	Status CollectionStatus  `json:"status"`
	Result *CollectionResult `json:"result,omitempty"`
	Error  string            `json:"error,omitempty"`
}

// SourceState carries one source's enabled flag inside SaveSettingsRequest.
type SourceState struct {
	ID      int64 `json:"id"`
	Enabled bool  `json:"enabled"`
}
