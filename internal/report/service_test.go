package report

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

type reportGeneratorStub struct {
	value   string
	err     error
	request summary.ReportRequest
	groups  []summary.ReportThreatGroup
}

func (g *reportGeneratorStub) GenerateReport(_ context.Context, request summary.ReportRequest) (summary.ReportResult, error) {
	g.request = request
	return summary.ReportResult{Summary: g.value, ThreatGroups: g.groups}, g.err
}

func TestServiceCreatePersistsGeneratedReport_whenArticlesExist(t *testing.T) {
	// Given a reportable article and a successful summary generator.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at, severity)
		VALUES (1, 'reportable', 'Critical incident', 'https://example.com', '2026-08-03',
		'2026-08-03T00:00:00Z', 'CRITICAL')`).Error; err != nil {
		t.Fatalf("insert article: %v", err)
	}
	repository := NewRepository(db)
	generator := &reportGeneratorStub{value: "Generated summary"}
	service := NewService(repository, generator)

	// When the service creates an English daily report.
	created, err := service.Create(context.Background(), api.CreateReportRequest{
		Type: "daily", Start: "2026-08-03", End: "2026-08-03",
	}, CreateOptions{Language: "en", TimezoneOffsetMinutes: 9 * 60})

	// Then the generated summary is persisted with the requested language route.
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	listed, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if created.Summary != "Generated summary" || len(listed) != 1 || listed[0].Summary != created.Summary {
		t.Fatalf("created = %+v, listed = %+v", created, listed)
	}
	if generator.request.Language != "en" || generator.request.Kind != "daily report" || len(generator.request.Facts) == 0 {
		t.Fatalf("generator request = %+v", generator.request)
	}
}

func TestServiceCreateDoesNotPersistReport_whenSummaryGenerationFails(t *testing.T) {
	// Given a reportable article and a failing summary generator.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
		VALUES (1, 'reportable', 'Incident', 'https://example.com', '2026-08-03', '2026-08-03T00:00:00Z')`).Error; err != nil {
		t.Fatalf("insert article: %v", err)
	}
	repository := NewRepository(db)
	generationError := errors.New("generator unavailable")
	service := NewService(repository, &reportGeneratorStub{err: generationError})

	// When report creation reaches summary generation.
	_, err = service.Create(context.Background(), api.CreateReportRequest{
		Type: "daily", Start: "2026-08-03", End: "2026-08-03",
	}, CreateOptions{Language: "ko", TimezoneOffsetMinutes: 9 * 60})

	// Then the error is preserved and no partial report is stored.
	if !errors.Is(err, generationError) {
		t.Fatalf("create error = %v, want generator error", err)
	}
	listed, listErr := repository.List(context.Background())
	if listErr != nil {
		t.Fatalf("list reports: %v", listErr)
	}
	if len(listed) != 0 {
		t.Fatalf("stored reports = %d, want 0", len(listed))
	}
}

func TestServiceCreatePersistsTranslatedLLMMergedThreats_whenSelectionIsValid(t *testing.T) {
	// Given three distinct report candidates and an LLM selection that identifies two as one incident.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	for index, title := range []string{"Major breach report", "Follow-up on the same breach", "Separate ransomware incident"} {
		if err := db.Create(&database.Article{
			SourceID: 1, FeedUID: fmt.Sprintf("selection-%d", index), Title: title,
			URL: fmt.Sprintf("https://example.com/selection-%d", index), PublishedAt: "2026-08-03",
			CollectedAt: "2026-08-03T00:00:00Z", Severity: "CRITICAL",
		}).Error; err != nil {
			t.Fatalf("insert article: %v", err)
		}
	}
	if err := db.Create(&database.DailySummary{Day: "2026-08-03", Summary: "Daily digest", GeneratedAt: "2026-08-03T23:00:00Z"}).Error; err != nil {
		t.Fatalf("insert daily summary: %v", err)
	}
	generator := &reportGeneratorStub{
		value: "Generated summary",
		groups: []summary.ReportThreatGroup{
			{RepresentativeID: "threat-1", MemberIDs: []string{"threat-1", "threat-2"}, TranslatedTitle: "대규모 정보 유출"},
			{RepresentativeID: "threat-3", MemberIDs: []string{"threat-3"}, TranslatedTitle: "별도의 랜섬웨어 사고"},
		},
	}

	// When the weekly report is created.
	created, err := NewService(NewRepository(db), generator).Create(context.Background(), api.CreateReportRequest{
		Type: "weekly", Start: "2026-08-01", End: "2026-08-07",
	}, CreateOptions{Language: "ko"})

	// Then the semantic duplicate is collapsed and its source count is retained.
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if len(created.TopThreats) != 2 || created.TopThreats[0].Title != "대규모 정보 유출" ||
		created.TopThreats[1].Title != "별도의 랜섬웨어 사고" || created.TopThreats[0].SourceCount != 2 ||
		created.TopThreat != created.TopThreats[0].Title {
		t.Fatalf("created top threats = %+v", created.TopThreats)
	}
}
