package feed

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestCollectorCollect_RecollectionDoesNotIncreaseCVEMentions(t *testing.T) {
	// Given a feed with two articles where only the first mentions a CVE.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	collector := NewCollector(repository, &selectedFeedStub{document: Document{
		Status: Status{OK: true},
		Articles: []FeedArticle{
			{ID: "article-a", URL: "https://example.com/a", Title: "CVE-2026-1001 affects A", PublishedAt: "2026-08-03T01:00:00Z"},
			{ID: "article-b", URL: "https://example.com/b", Title: "Article B", PublishedAt: "2026-08-03T02:00:00Z"},
		},
	}}, &articleBodyStub{})
	if _, err := collector.Collect(context.Background(), "2026-08-03"); err != nil {
		t.Fatalf("collect day: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO article_cves (article_id, cve_id)
		SELECT id, 'CVE-2026-1001' FROM articles WHERE feed_uid = 'article-b'`); err != nil {
		t.Fatalf("seed incorrect CVE link: %v", err)
	}

	// When the same day is collected again.
	if _, err := collector.Collect(context.Background(), "2026-08-03"); err != nil {
		t.Fatalf("recollect day: %v", err)
	}

	// Then the CVE remains linked only to the article that mentions it.
	var mentions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_cves WHERE cve_id = 'CVE-2026-1001'`).Scan(&mentions); err != nil {
		t.Fatalf("count CVE mentions: %v", err)
	}
	if mentions != 1 {
		t.Fatalf("CVE mentions = %d, want 1 after recollection", mentions)
	}
	var unrelatedLinks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM article_cves ac
		JOIN articles a ON a.id = ac.article_id WHERE a.feed_uid = 'article-b'`).Scan(&unrelatedLinks); err != nil {
		t.Fatalf("count unrelated CVE links: %v", err)
	}
	if unrelatedLinks != 0 {
		t.Fatalf("unrelated article CVE links = %d, want 0", unrelatedLinks)
	}
}
