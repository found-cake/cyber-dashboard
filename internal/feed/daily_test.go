package feed

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestDailyReturnsMultipleArticles_whenPoolHasOneConnection(t *testing.T) {
	// Given two articles with CVEs and a one-connection SQLite pool.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	source := api.Source{ID: 1}
	for index, article := range []FeedArticle{
		{ID: "sha256:first", URL: "https://example.com/first", Title: "First CVE-2026-1001", PublishedAt: "2026-08-02T01:00:00Z"},
		{ID: "sha256:second", URL: "https://example.com/second", Title: "Second CVE-2026-1002", PublishedAt: "2026-08-02T02:00:00Z"},
	} {
		if err := repository.SaveArticle(context.Background(), source, article, "2026-08-02"); err != nil {
			t.Fatalf("save article %d: %v", index, err)
		}
	}

	// When the day is loaded through that pool.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	daily, err := repository.Daily(ctx, "2026-08-02")

	// Then outer rows release the connection before nested CVE reads.
	if err != nil {
		t.Fatalf("load daily: %v", err)
	}
	if len(daily.Articles) != 2 {
		t.Fatalf("article count = %d, want 2", len(daily.Articles))
	}
}
