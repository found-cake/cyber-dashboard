package report

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

type reportGeneratorStub struct {
	value   string
	err     error
	request summary.Request
}

func (g *reportGeneratorStub) Generate(_ context.Context, request summary.Request) (string, error) {
	g.request = request
	return g.value, g.err
}

func TestServiceCreatePersistsGeneratedReport_whenArticlesExist(t *testing.T) {
	// Given a reportable article and a successful summary generator.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at, severity)
		VALUES (1, 'reportable', 'Critical incident', 'https://example.com', '2026-08-03',
		'2026-08-03T00:00:00Z', 'CRITICAL')`); err != nil {
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
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
		VALUES (1, 'reportable', 'Incident', 'https://example.com', '2026-08-03', '2026-08-03T00:00:00Z')`); err != nil {
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
