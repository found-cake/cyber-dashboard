package collector_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
	"github.com/found-cake/cyber-dashboard/internal/feed/store"
)

type feedStub struct {
	document collector.Document
}

func (stub *feedStub) Fetch(context.Context, api.Source) (collector.Document, error) {
	return stub.document, nil
}

type bodyStub struct{}

func (*bodyStub) Load(context.Context, api.Source, collector.FeedArticle) (string, error) {
	return "", nil
}

type articleRepository struct {
	source api.Source
	saved  []collector.FeedArticle
}

func (repository *articleRepository) Sources(context.Context) ([]api.Source, error) {
	return []api.Source{repository.source}, nil
}

func (repository *articleRepository) SaveArticle(_ context.Context, _ api.Source, article collector.FeedArticle, _ string) error {
	repository.saved = append(repository.saved, article)
	return nil
}

func TestCollectorCollect_SavesOnlyBoanNewsIncidentArticles(t *testing.T) {
	// Given an enabled BoanNews feed with incident and non-incident categories.
	repository := &articleRepository{source: api.Source{Name: "보안뉴스", Slug: "boannews", Enabled: true}}
	service := collector.NewCollector(repository, &feedStub{document: collector.Document{
		Status: collector.Status{OK: true},
		Articles: []collector.FeedArticle{
			{ID: "incident", URL: "https://example.com/incident", PublishedAt: "2026-08-03T01:00:00Z", Categories: []string{"국제", "사건·사고"}},
			{ID: "opinion", URL: "https://example.com/opinion", PublishedAt: "2026-08-03T02:00:00Z", Categories: []string{"오피니언"}},
		},
	}}, &bodyStub{})

	// When the feed is collected.
	if _, err := service.Collect(context.Background(), "2026-08-03"); err != nil {
		t.Fatalf("collect day: %v", err)
	}

	// Then only the article with the 사건·사고 category is saved.
	if len(repository.saved) != 1 {
		t.Fatalf("saved articles = %d, want 1", len(repository.saved))
	}
	if repository.saved[0].ID != "incident" {
		t.Fatalf("saved article = %q, want incident", repository.saved[0].ID)
	}
}

func TestCollectorCollect_RecollectionDoesNotIncreaseCVEMentions(t *testing.T) {
	// Given a feed with two articles where only the first mentions a CVE.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := store.NewRepository(db)
	service := collector.NewCollector(repository, &feedStub{document: collector.Document{
		Status: collector.Status{OK: true},
		Articles: []collector.FeedArticle{
			{ID: "article-a", URL: "https://example.com/a", Title: "CVE-2026-1001 affects A", PublishedAt: "2026-08-03T01:00:00Z"},
			{ID: "article-b", URL: "https://example.com/b", Title: "Article B", PublishedAt: "2026-08-03T02:00:00Z"},
		},
	}}, &bodyStub{})
	if _, err := service.Collect(context.Background(), "2026-08-03"); err != nil {
		t.Fatalf("collect day: %v", err)
	}
	if err := db.Exec(`INSERT INTO article_cves (article_id, cve_id)
		SELECT id, 'CVE-2026-1001' FROM articles WHERE feed_uid = 'article-b'`).Error; err != nil {
		t.Fatalf("seed incorrect CVE link: %v", err)
	}

	// When the same day is collected again.
	if _, err := service.Collect(context.Background(), "2026-08-03"); err != nil {
		t.Fatalf("recollect day: %v", err)
	}

	// Then the CVE remains linked only to the article that mentions it.
	var mentions int
	if err := db.Raw(`SELECT COUNT(*) FROM article_cves WHERE cve_id = 'CVE-2026-1001'`).Row().Scan(&mentions); err != nil {
		t.Fatalf("count CVE mentions: %v", err)
	}
	if mentions != 1 {
		t.Fatalf("CVE mentions = %d, want 1 after recollection", mentions)
	}
	var unrelatedLinks int
	if err := db.Raw(`SELECT COUNT(*) FROM article_cves ac
		JOIN articles a ON a.id = ac.article_id WHERE a.feed_uid = 'article-b'`).Row().Scan(&unrelatedLinks); err != nil {
		t.Fatalf("count unrelated CVE links: %v", err)
	}
	if unrelatedLinks != 0 {
		t.Fatalf("unrelated article CVE links = %d, want 0", unrelatedLinks)
	}
}
