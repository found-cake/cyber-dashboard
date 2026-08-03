package feed

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
)

type analysisStub struct {
	request summary.ArticleRequest
	result  summary.ArticleAnalysis
}

func (s *analysisStub) AnalyzeArticle(_ context.Context, request summary.ArticleRequest) (summary.ArticleAnalysis, error) {
	s.request = request
	return s.result, nil
}

func TestSaveArticleAnalysisUsesImpactSeverity_whenNoHigherCVSSExists(t *testing.T) {
	// Given a stored article with a CVE and explicit contextual impact returned by AI.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	if err := repository.SaveArticle(context.Background(), api.Source{ID: 1}, FeedArticle{
		ID: "sha256:impact", URL: "https://example.com/impact", Title: "Incident CVE-2026-1001",
		PublishedAt: "2026-08-03T01:00:00Z", Body: "Full incident body",
	}, "2026-08-03"); err != nil {
		t.Fatalf("save article: %v", err)
	}
	var articleID int64
	if err := db.QueryRow(`SELECT id FROM articles WHERE feed_uid = ?`, "sha256:impact").Scan(&articleID); err != nil {
		t.Fatalf("read article id: %v", err)
	}

	// When analysis reports 15,000 victims without a zero-day.
	err = repository.SaveArticleAnalysis(context.Background(), articleID, summary.ArticleAnalysis{
		Summary: "Analyzed summary", AttackMethod: "Supply chain", ThreatActor: "Actor",
		TargetSector: "IT", VictimCount: 15_000,
	})

	// Then contextual impact raises severity to HIGH and classification fields are persisted.
	if err != nil {
		t.Fatalf("save article analysis: %v", err)
	}
	var articleSummary, severity string
	var victims int
	if err := db.QueryRow(`SELECT summary, severity, victim_count FROM articles WHERE id = ?`, articleID).Scan(&articleSummary, &severity, &victims); err != nil {
		t.Fatalf("read analyzed article: %v", err)
	}
	if articleSummary != "Analyzed summary" || severity != "HIGH" || victims != 15_000 {
		t.Fatalf("summary = %q, severity = %q, victims = %d", articleSummary, severity, victims)
	}
}

func TestSaveArticleStartsStepSecurityThreatIntelAtHigh_whenImpactSignalsUnavailable(t *testing.T) {
	// Given the StepSecurity source and a filtered-in Threat Intel article without CVSS or victim signals.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var source api.Source
	if err := db.QueryRow(`SELECT id, slug FROM sources WHERE slug = 'stepsecurity'`).Scan(&source.ID, &source.Slug); err != nil {
		t.Fatalf("read StepSecurity source: %v", err)
	}
	repository := NewRepository(db)

	// When the Threat Intel article is first stored.
	err = repository.SaveArticle(context.Background(), source, FeedArticle{
		ID: "sha256:step-threat", URL: "https://www.stepsecurity.io/blog/threat", Title: "Supply-chain report",
		PublishedAt: "2026-08-03T01:00:00Z", Body: "Threat intelligence without a CVE score",
	}, "2026-08-03")

	// Then its source policy supplies a HIGH initial severity.
	if err != nil {
		t.Fatalf("save article: %v", err)
	}
	var got string
	if err := db.QueryRow(`SELECT severity FROM articles WHERE feed_uid = 'sha256:step-threat'`).Scan(&got); err != nil {
		t.Fatalf("read article severity: %v", err)
	}
	if got != "HIGH" {
		t.Fatalf("severity = %q, want HIGH", got)
	}
}

func TestSaveArticleAnalysisKeepsStepSecurityThreatIntelHigh_whenLLMSignalsUnknown(t *testing.T) {
	// Given a stored StepSecurity Threat Intel article without CVSS information.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var source api.Source
	if err := db.QueryRow(`SELECT id, slug FROM sources WHERE slug = 'stepsecurity'`).Scan(&source.ID, &source.Slug); err != nil {
		t.Fatalf("read StepSecurity source: %v", err)
	}
	repository := NewRepository(db)
	if err := repository.SaveArticle(context.Background(), source, FeedArticle{
		ID: "sha256:step-analysis", URL: "https://www.stepsecurity.io/blog/threat-analysis", Title: "Supply-chain analysis",
		PublishedAt: "2026-08-03T01:00:00Z", Body: "Threat intelligence without explicit victims",
	}, "2026-08-03"); err != nil {
		t.Fatalf("save article: %v", err)
	}
	var articleID int64
	if err := db.QueryRow(`SELECT id FROM articles WHERE feed_uid = 'sha256:step-analysis'`).Scan(&articleID); err != nil {
		t.Fatalf("read article id: %v", err)
	}

	// When AI enrichment reports no numeric impact signals.
	err = repository.SaveArticleAnalysis(context.Background(), articleID, summary.ArticleAnalysis{
		Summary: "Supply-chain intelligence", AttackMethod: "Supply chain", ThreatActor: "Unknown", TargetSector: "Software",
	})

	// Then recalculation retains the StepSecurity HIGH floor.
	if err != nil {
		t.Fatalf("save analysis: %v", err)
	}
	var got string
	if err := db.QueryRow(`SELECT severity FROM articles WHERE id = ?`, articleID).Scan(&got); err != nil {
		t.Fatalf("read article severity: %v", err)
	}
	if got != "HIGH" {
		t.Fatalf("severity = %q, want HIGH", got)
	}
}

func TestSaveAssessmentKeepsCriticalContext_whenCVSSIsLower(t *testing.T) {
	// Given an article classified as an actively exploited zero-day.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	if err := repository.SaveArticle(context.Background(), api.Source{ID: 1}, FeedArticle{
		ID: "sha256:zero-day", URL: "https://example.com/zero-day", Title: "CVE-2026-2002",
		PublishedAt: "2026-08-03T01:00:00Z", Body: "Zero-day exploitation",
	}, "2026-08-03"); err != nil {
		t.Fatalf("save article: %v", err)
	}
	var articleID int64
	if err := db.QueryRow(`SELECT id FROM articles WHERE feed_uid = ?`, "sha256:zero-day").Scan(&articleID); err != nil {
		t.Fatalf("read article id: %v", err)
	}
	if err := repository.SaveArticleAnalysis(context.Background(), articleID, summary.ArticleAnalysis{
		Summary: "Zero-day summary", AttackMethod: "Exploit", ThreatActor: "Actor",
		TargetSector: "Government", ZeroDay: true,
	}); err != nil {
		t.Fatalf("save article analysis: %v", err)
	}

	// When a lower NIST CVSS score is saved afterward.
	err = repository.SaveAssessment(context.Background(), vulnerability.Assessment{
		CVEID: "CVE-2026-2002", Score: 5.5, Source: "nvd@nist.gov", Version: "3.1", Product: "acme / app",
	})

	// Then the combined result remains CRITICAL because context is a separate signal.
	if err != nil {
		t.Fatalf("save assessment: %v", err)
	}
	var severity string
	if err := db.QueryRow(`SELECT severity FROM articles WHERE id = ?`, articleID).Scan(&severity); err != nil {
		t.Fatalf("read severity: %v", err)
	}
	if severity != "CRITICAL" {
		t.Fatalf("severity = %q, want CRITICAL", severity)
	}
}

func TestArticleEnrichmentServiceSendsStoredFullBody_toConfiguredAI(t *testing.T) {
	// Given a stored full article and an AI analyzer.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	if err := repository.SaveArticle(context.Background(), api.Source{ID: 1}, FeedArticle{
		ID: "sha256:full-ai", URL: "https://example.com/full-ai", Title: "Full article",
		PublishedAt: "2026-08-03T01:00:00Z", Body: "Opening.\n\nFULL_BODY_END_MARKER",
	}, "2026-08-03"); err != nil {
		t.Fatalf("save article: %v", err)
	}
	analyzer := &analysisStub{result: summary.ArticleAnalysis{
		Summary: "Summary", AttackMethod: "Exploit", ThreatActor: "Actor", TargetSector: "IT",
	}}
	service := NewArticleEnrichmentService(repository, analyzer)

	// When the day is enriched.
	err = service.EnrichDay(context.Background(), "2026-08-03", "en")

	// Then the exact stored body reaches the AI request.
	if err != nil {
		t.Fatalf("enrich day: %v", err)
	}
	if analyzer.request.Body != "Opening.\n\nFULL_BODY_END_MARKER" {
		t.Fatalf("AI body = %q", analyzer.request.Body)
	}
}
