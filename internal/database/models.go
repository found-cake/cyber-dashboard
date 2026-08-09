package database

type Source struct {
	ID      int64  `gorm:"primaryKey;autoIncrement"`
	Name    string `gorm:"not null"`
	Host    string `gorm:"not null"`
	Slug    string `gorm:"not null;uniqueIndex"`
	Enabled bool   `gorm:"not null"`
}

type Article struct {
	ID             int64  `gorm:"primaryKey;autoIncrement;index:articles_published_at_time_id_idx,priority:3"`
	SourceID       int64  `gorm:"not null"`
	Source         Source `gorm:"constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
	FeedUID        string `gorm:"not null;uniqueIndex"`
	Title          string `gorm:"not null"`
	URL            string `gorm:"not null"`
	PublishedAt    string `gorm:"index:articles_published_at_time_id_idx,priority:1"`
	PublishedTime  string `gorm:"not null;default:'';index:articles_published_at_time_id_idx,priority:2"`
	CollectedAt    string `gorm:"not null"`
	Body           string `gorm:"not null;default:''"`
	Summary        string `gorm:"not null;default:''"`
	AttackMethod   string `gorm:"not null;default:Unclassified"`
	ThreatActor    string `gorm:"not null;default:Unknown"`
	ActorCountry   string `gorm:"not null;default:''"`
	Sector         string `gorm:"not null;default:일반"`
	VictimCount    int    `gorm:"not null;default:0"`
	DamageUSD      int64  `gorm:"not null;default:0"`
	ZeroDay        bool   `gorm:"not null;default:false"`
	PatchAvailable string `gorm:"not null;default:''"`
	Severity       string `gorm:"not null;default:UNKNOWN"`
}

type DailySummary struct {
	Day         string `gorm:"primaryKey"`
	Summary     string `gorm:"not null"`
	GeneratedAt string `gorm:"not null"`
}

type CVE struct {
	CVEID           string  `gorm:"primaryKey"`
	FirstSeen       string  `gorm:"not null"`
	CVSSScore       float64 `gorm:"not null;default:0"`
	CVSSSource      string  `gorm:"not null;default:''"`
	CVSSVersion     string  `gorm:"not null;default:''"`
	CVSSVector      string  `gorm:"not null;default:''"`
	AffectedProduct string  `gorm:"not null;default:NVD enrichment pending"`
}

type RejectedCVE struct {
	CVEID string `gorm:"primaryKey"`
}

type ArticleCVE struct {
	ArticleID int64   `gorm:"primaryKey;not null;index:article_cves_cve_id_article_id_idx,priority:2"`
	Article   Article `gorm:"constraint:OnUpdate:NO ACTION,OnDelete:CASCADE"`
	CVEID     string  `gorm:"primaryKey;not null;index:article_cves_cve_id_article_id_idx,priority:1"`
	CVE       CVE     `gorm:"constraint:OnUpdate:NO ACTION,OnDelete:CASCADE"`
}

type Report struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Type        string `gorm:"not null"`
	PeriodStart string `gorm:"not null"`
	PeriodEnd   string `gorm:"not null"`
	Total       int    `gorm:"not null"`
	Critical    int    `gorm:"not null"`
	High        int    `gorm:"not null"`
	Medium      int    `gorm:"not null"`
	TopThreat   string `gorm:"not null"`
	Actors      string `gorm:"not null"`
	Sectors     string `gorm:"not null"`
	Summary     string `gorm:"not null"`
	GeneratedAt string `gorm:"not null"`
}

type Setting struct {
	ID                    int64  `gorm:"primaryKey;check:id = 1"`
	Lang                  string `gorm:"not null;default:ko"`
	Theme                 string `gorm:"not null;default:dark"`
	Accent                string `gorm:"not null;default:#4f6ef7"`
	LLMBaseURL            string `gorm:"not null;default:https://api.openai.com/v1"`
	LLMModel              string `gorm:"not null;default:gpt-4o-mini"`
	LLMAPIKey             string `gorm:"not null;default:''"`
	LLMTimeout            int    `gorm:"not null;default:60"`
	NVDAPIKey             string `gorm:"not null;default:''"`
	TimezoneOffsetMinutes *int
}

type LLMPreset struct {
	ID      int64  `gorm:"primaryKey;autoIncrement"`
	Label   string `gorm:"not null"`
	BaseURL string `gorm:"not null;uniqueIndex:llm_presets_endpoint_model_idx,priority:1"`
	Model   string `gorm:"not null;uniqueIndex:llm_presets_endpoint_model_idx,priority:2"`
	APIKey  string `gorm:"not null;default:''"`
	Builtin bool   `gorm:"not null;default:false"`
}
