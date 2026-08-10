package feed

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestSetSourcesEnabledUpdatesSelectedSources_whenSourcesExist(t *testing.T) {
	// Given a repository with the seeded enabled sources.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)

	// When one source is enabled and another disabled in the same call.
	if err := repository.SetSourcesEnabled(context.Background(), []api.SourceState{{ID: 1, Enabled: true}, {ID: 2, Enabled: false}}); err != nil {
		t.Fatalf("update sources: %v", err)
	}
	sources, err := repository.Sources(context.Background())

	// Then only the selected sources reflect the new values.
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) < 3 || !sources[0].Enabled || sources[1].Enabled || !sources[2].Enabled {
		t.Fatalf("sources = %+v", sources)
	}
}

func TestSetSourcesEnabledRollsBack_whenOneSourceDoesNotExist(t *testing.T) {
	// Given a repository without the requested source ID.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)

	// When a valid change is submitted alongside an unknown source.
	err = repository.SetSourcesEnabled(context.Background(), []api.SourceState{{ID: 2, Enabled: false}, {ID: 999, Enabled: false}})

	// Then callers receive the stable not-found error and the valid change is discarded.
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("update error = %v, want ErrNotFound", err)
	}
	sources, err := repository.Sources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) < 2 || !sources[1].Enabled {
		t.Fatalf("sources = %+v, want the second source still enabled", sources)
	}
}

func TestSaveDailySummaryReplacesValue_whenDayIsRegenerated(t *testing.T) {
	// Given a saved daily summary.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)
	if err := repository.SaveDailySummary(context.Background(), "2026-08-03", "First summary"); err != nil {
		t.Fatalf("save first summary: %v", err)
	}

	// When the same day is regenerated.
	if err := repository.SaveDailySummary(context.Background(), "2026-08-03", "Updated summary"); err != nil {
		t.Fatalf("save updated summary: %v", err)
	}
	daily, err := repository.Daily(context.Background(), "2026-08-03")

	// Then the day exposes only the latest summary.
	if err != nil {
		t.Fatalf("load daily data: %v", err)
	}
	if daily.Summary != "Updated summary" {
		t.Fatalf("daily summary = %q, want updated value", daily.Summary)
	}
}

func TestCollectedDaysReturnsDistinctAscendingDates_whenArticlesExist(t *testing.T) {
	// Given articles saved on repeated and out-of-order dates.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)
	for _, article := range []struct {
		id, publishedAt, day string
	}{
		{id: "later", publishedAt: "2026-08-03T02:00:00Z", day: "2026-08-03"},
		{id: "earlier", publishedAt: "2026-08-01T01:00:00Z", day: "2026-08-01"},
		{id: "same-day", publishedAt: "2026-08-03T03:00:00Z", day: "2026-08-03"},
	} {
		if err := repository.SaveArticle(context.Background(), api.Source{ID: 1}, FeedArticle{
			ID: article.id, URL: "https://example.com/" + article.id, Title: article.id, PublishedAt: article.publishedAt,
		}, article.day); err != nil {
			t.Fatalf("save article %q: %v", article.id, err)
		}
	}

	// When collected dates are queried.
	days, err := repository.CollectedDays(context.Background())

	// Then dates are unique and ascending.
	if err != nil {
		t.Fatalf("collect days: %v", err)
	}
	if len(days) != 2 || days[0] != "2026-08-01" || days[1] != "2026-08-03" {
		t.Fatalf("days = %v", days)
	}
}

func TestSaveArticleUsesEnglishPlaceholders_whenClassificationIsUnavailable(t *testing.T) {
	// Given an article without a feed category or AI analysis.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)

	// When the article is saved through the production repository.
	err = repository.SaveArticle(context.Background(), api.Source{ID: 1}, FeedArticle{
		ID: "unclassified", URL: "https://example.com/unclassified", Title: "Unclassified article",
	}, "2026-08-06")
	if err != nil {
		t.Fatalf("save article: %v", err)
	}

	// Then both classification placeholders use stable English labels.
	var attackMethod, threatActor string
	if err := db.Raw(`SELECT attack_method, threat_actor FROM articles WHERE feed_uid = ?`, "unclassified").Row().Scan(&attackMethod, &threatActor); err != nil {
		t.Fatalf("read saved classifications: %v", err)
	}
	if attackMethod != "Unclassified" || threatActor != "Unknown" {
		t.Fatalf("classifications = %q, %q, want Unclassified, Unknown", attackMethod, threatActor)
	}
}
