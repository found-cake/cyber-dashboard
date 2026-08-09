package report

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestBuildReturnsNotFoundForEmptyPeriod(t *testing.T) {
	// Given a new database with no articles.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	// When a report is built for an empty period.
	_, _, err = NewRepository(db).Build(context.Background(), api.CreateReportRequest{
		Type: "weekly", Start: "2026-08-01", End: "2026-08-03",
	})

	// Then callers can map the stable not-found error to HTTP 404.
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("build error = %v, want ErrNotFound", err)
	}
}

func TestBuildAggregatesReportData_whenPeriodContainsArticles(t *testing.T) {
	// Given reportable articles with distinct severity, actor, and sector data.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	rows := []struct {
		uid, title, severity, actor, sector, summary string
	}{
		{uid: "critical", title: "Critical incident", severity: "CRITICAL", actor: "Group A", sector: "Finance", summary: "Critical facts"},
		{uid: "high", title: "High incident", severity: "HIGH", actor: "Group A", sector: "Energy", summary: "High facts"},
		{uid: "medium", title: "Medium incident", severity: "MEDIUM", actor: "Group B", sector: "Finance", summary: "Medium facts"},
	}
	for _, row := range rows {
		if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at,
			severity, threat_actor, sector, summary) VALUES (1, ?, ?, 'https://example.com', '2026-08-02',
			'2026-08-02T00:00:00Z', ?, ?, ?, ?)`, row.uid, row.title, row.severity, row.actor, row.sector, row.summary).Error; err != nil {
			t.Fatalf("insert article %q: %v", row.uid, err)
		}
	}

	// When a report is built for the populated period.
	value, facts, err := NewRepository(db).Build(context.Background(), api.CreateReportRequest{
		Type: "daily", Start: "2026-08-02", End: "2026-08-02",
	})

	// Then its aggregate and ranking fields reflect the stored articles.
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if value.Total != 3 || value.Critical != 1 || value.High != 1 || value.Medium != 1 || value.TopThreat != "Critical incident" {
		t.Fatalf("report = %+v", value)
	}
	if len(value.Actors) != 2 || value.Actors[0] != "Group A" || len(value.Sectors) != 2 || len(facts) != 3 {
		t.Fatalf("actors = %v, sectors = %v, facts = %v", value.Actors, value.Sectors, facts)
	}
}

func TestBuildTreatsPeriodAsData_whenPeriodContainsSQLSyntax(t *testing.T) {
	// Given one stored article and a report period containing SQL syntax.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
		VALUES (1, 'safe', 'Stored article', 'https://example.com', '2026-08-02', '2026-08-02T00:00:00Z')`).Error; err != nil {
		t.Fatalf("insert article: %v", err)
	}

	// When the untrusted period reaches the report query.
	_, _, err = NewRepository(db).Build(context.Background(), api.CreateReportRequest{
		Type: "daily", Start: "' OR 1=1 --", End: "' OR 1=1 --",
	})

	// Then the syntax is treated as a value and cannot select the stored row.
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("build error = %v, want ErrNotFound", err)
	}
}

func TestTopValuesRejectsColumnOutsideAllowlist(t *testing.T) {
	// Given a repository query whose column contains SQL syntax.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	// When the column reaches the top-values boundary.
	_, err = NewRepository(db).topValues(context.Background(), topQuery{
		column: "sector); DROP TABLE reports; --", period: valuePeriod{start: "2026-08-01", end: "2026-08-03"}, limit: 5,
	})

	// Then it is rejected before a SQL statement is selected.
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("top values error = %v, want invalid column", err)
	}
}

func TestSaveAndListPreserveReport_whenTimezoneIsConfigured(t *testing.T) {
	// Given a report repository with a fixed UTC clock.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)
	repository.now = func() time.Time { return time.Date(2026, 8, 4, 1, 2, 3, 0, time.UTC) }
	input := api.Report{
		Type: "daily", PeriodStart: "2026-08-03", PeriodEnd: "2026-08-03", Total: 3,
		Critical: 1, High: 1, Medium: 1, TopThreat: "Critical incident",
		Actors: []string{"Group A", "Group B"}, Sectors: []string{"Finance", "Energy"}, Summary: "Daily summary",
	}

	// When the report is saved at UTC+9 and listed again.
	saved, err := repository.Save(context.Background(), input, 9*60)
	if err != nil {
		t.Fatalf("save report: %v", err)
	}
	listed, err := repository.List(context.Background())

	// Then its configured timestamp and persisted fields round-trip.
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if saved.GeneratedAt != "2026-08-04T10:02:03+09:00" || len(listed) != 1 {
		t.Fatalf("saved = %+v, listed = %+v", saved, listed)
	}
	if listed[0].Summary != input.Summary || strings.Join(listed[0].Actors, ",") != "Group A,Group B" || strings.Join(listed[0].Sectors, ",") != "Finance,Energy" {
		t.Fatalf("listed report = %+v", listed[0])
	}
}

func TestSaveAndListPreserveReportEntries_whenActorsContainCommas(t *testing.T) {
	// Given actor and sector names that contain commas, as LLM analysis produces.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)
	actors := []string{"Scattered Spider, Inc.", "Lazarus Group"}
	sectors := []string{"금융, 보험", "정부"}

	// When the report is saved and listed again.
	if _, err := repository.Save(context.Background(), api.Report{
		Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07",
		Actors: actors, Sectors: sectors, Summary: "Weekly summary",
	}, 0); err != nil {
		t.Fatalf("save report: %v", err)
	}
	listed, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}

	// Then each name stays one entry instead of splitting on its own comma.
	if len(listed) != 1 {
		t.Fatalf("listed %d reports, want 1", len(listed))
	}
	if !slices.Equal(listed[0].Actors, actors) {
		t.Fatalf("listed actors = %q, want %q", listed[0].Actors, actors)
	}
	if !slices.Equal(listed[0].Sectors, sectors) {
		t.Fatalf("listed sectors = %q, want %q", listed[0].Sectors, sectors)
	}
}

func TestDeleteRemovesReport_whenIDExists(t *testing.T) {
	// Given a stored report.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)
	saved, err := repository.Save(context.Background(), api.Report{
		Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07",
		Actors: []string{}, Sectors: []string{}, Summary: "Disposable report",
	}, 0)
	if err != nil {
		t.Fatalf("save report: %v", err)
	}

	// When the stored report is deleted by ID.
	err = repository.Delete(context.Background(), saved.ID)

	// Then it is absent from the report list.
	if err != nil {
		t.Fatalf("delete report: %v", err)
	}
	listed, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed reports = %d, want 0", len(listed))
	}
}

func TestDeleteReturnsNotFound_whenIDDoesNotExist(t *testing.T) {
	// Given a report repository with no stored reports.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	// When an unknown report ID is deleted.
	err = NewRepository(db).Delete(context.Background(), 999999)

	// Then callers can map the stable not-found error to HTTP 404.
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete error = %v, want ErrNotFound", err)
	}
}
