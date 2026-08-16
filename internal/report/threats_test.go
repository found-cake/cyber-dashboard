package report

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestBuildGroupsOnlyHighConfidenceDuplicateThreats(t *testing.T) {
	// Given duplicate titles, an overlapping CVE, and two distinct incidents by the same actor.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	articles := []database.Article{
		{SourceID: 1, FeedUID: "sogang-a", Title: "서강대 개인정보 18만건 유출... 박근혜 전 대통령 포함", URL: "https://news.example/sogang?utm_source=rss", PublishedAt: "2026-08-15", CollectedAt: "2026-08-15T01:00:00Z", Summary: "대학 개인정보 유출", ThreatActor: "Unknown", Severity: "CRITICAL", VictimCount: 180000},
		{SourceID: 1, FeedUID: "sogang-b", Title: "서강대 개인정보 18만건 유출 박근혜 전 대통령 포함", URL: "https://other.example/sogang", PublishedAt: "2026-08-15", CollectedAt: "2026-08-15T02:00:00Z", Summary: "같은 개인정보 유출", ThreatActor: "Unknown", Severity: "CRITICAL", VictimCount: 180000},
		{SourceID: 1, FeedUID: "cve-a", Title: "Gateway attacks observed in the wild", URL: "https://news.example/gateway-a", PublishedAt: "2026-08-14", CollectedAt: "2026-08-14T01:00:00Z", Summary: "Gateway exploitation", ThreatActor: "Group A", Severity: "CRITICAL"},
		{SourceID: 1, FeedUID: "cve-b", Title: "Emergency patch follows gateway exploitation", URL: "https://news.example/gateway-b", PublishedAt: "2026-08-14", CollectedAt: "2026-08-14T02:00:00Z", Summary: "Same gateway exploitation", ThreatActor: "Group B", Severity: "HIGH"},
		{SourceID: 1, FeedUID: "actor-a", Title: "Group Z compromises a finance provider", URL: "https://news.example/finance", PublishedAt: "2026-08-13", CollectedAt: "2026-08-13T01:00:00Z", Summary: "Finance intrusion", ThreatActor: "Group Z", Severity: "HIGH"},
		{SourceID: 1, FeedUID: "actor-b", Title: "Group Z launches a separate telecom intrusion", URL: "https://news.example/telecom", PublishedAt: "2026-08-12", CollectedAt: "2026-08-12T01:00:00Z", Summary: "Telecom intrusion", ThreatActor: "Group Z", Severity: "HIGH"},
	}
	for index := range articles {
		if err := db.Create(&articles[index]).Error; err != nil {
			t.Fatalf("insert article %q: %v", articles[index].FeedUID, err)
		}
	}
	for _, day := range []string{"2026-08-12", "2026-08-13", "2026-08-14", "2026-08-15"} {
		if err := db.Create(&database.DailySummary{Day: day, Summary: "Daily digest " + day, GeneratedAt: day + "T23:00:00Z"}).Error; err != nil {
			t.Fatalf("insert daily summary %s: %v", day, err)
		}
	}
	if err := db.Create(&database.CVE{CVEID: "CVE-2026-4242", FirstSeen: "2026-08-14"}).Error; err != nil {
		t.Fatalf("insert CVE: %v", err)
	}
	for _, article := range articles[2:4] {
		if err := db.Create(&database.ArticleCVE{ArticleID: article.ID, CVEID: "CVE-2026-4242"}).Error; err != nil {
			t.Fatalf("link CVE: %v", err)
		}
	}

	// When a weekly report draft is built.
	draft, err := NewRepository(db).Build(context.Background(), api.CreateReportRequest{
		Type: "weekly", Start: "2026-08-09", End: "2026-08-15",
	})

	// Then only the critical title and CVE groups remain; high-only incidents are excluded.
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if len(draft.threats) != 2 {
		t.Fatalf("threat candidates = %+v, want 2", draft.threats)
	}
	counts := map[string]int{}
	for _, threat := range draft.threats {
		counts[threat.title] = threat.sourceCount
	}
	if counts[articles[0].Title] != 2 || counts[articles[2].Title] != 2 {
		t.Fatalf("grouped source counts = %+v", counts)
	}
}

func TestBuildBoundsLLMCandidatesToTwiceTheReportLimit(t *testing.T) {
	// Given more distinct incidents than either report type may send to the LLM.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	for index := 0; index < 24; index++ {
		article := database.Article{
			SourceID: 1, FeedUID: fmt.Sprintf("incident-%02d", index), Title: fmt.Sprintf("Distinct incident number %02d", index),
			URL: fmt.Sprintf("https://news.example/%02d", index), PublishedAt: "2026-08-15", CollectedAt: "2026-08-15T01:00:00Z", Severity: "CRITICAL",
		}
		if err := db.Create(&article).Error; err != nil {
			t.Fatalf("insert article %d: %v", index, err)
		}
	}
	if err := db.Create(&database.DailySummary{Day: "2026-08-15", Summary: "Daily digest", GeneratedAt: "2026-08-15T23:00:00Z"}).Error; err != nil {
		t.Fatalf("insert daily summary: %v", err)
	}
	repository := NewRepository(db)

	// When weekly and monthly drafts are built from the same period.
	weekly, weeklyErr := repository.Build(context.Background(), api.CreateReportRequest{Type: "weekly", Start: "2026-08-01", End: "2026-08-15"})
	monthly, monthlyErr := repository.Build(context.Background(), api.CreateReportRequest{Type: "monthly", Start: "2026-08-01", End: "2026-08-15"})

	// Then their candidate pools are bounded at six and twenty before reaching the LLM.
	if weeklyErr != nil || monthlyErr != nil {
		t.Fatalf("build weekly = %v, monthly = %v", weeklyErr, monthlyErr)
	}
	if len(weekly.threats) != 6 || len(monthly.threats) != 20 {
		t.Fatalf("candidate counts weekly=%d monthly=%d", len(weekly.threats), len(monthly.threats))
	}
}
