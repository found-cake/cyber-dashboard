package feed

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

type browserBodyStub struct {
	body  string
	calls int
}

type articleBodyStub struct {
	body string
}

func (s *articleBodyStub) Load(context.Context, api.Source, FeedArticle) (string, error) {
	return s.body, nil
}

type selectedFeedStub struct {
	document Document
}

type allSourcesFeedStub struct {
	document Document
}

type filteredArticleBodyStub struct{}

func (s *selectedFeedStub) Fetch(_ context.Context, source api.Source) (Document, error) {
	if source.Slug == "boannews" {
		return s.document, nil
	}
	return Document{}, nil
}

func (s *allSourcesFeedStub) Fetch(context.Context, api.Source) (Document, error) {
	return s.document, nil
}

func (*filteredArticleBodyStub) Load(context.Context, api.Source, FeedArticle) (string, error) {
	return "", ErrArticleFiltered
}

func (s *browserBodyStub) Load(_ context.Context, _ string) (string, error) {
	s.calls++
	return s.body, nil
}

func TestArticleBodyLoaderUsesEmbeddedContent_whenFeedContainsFullArticle(t *testing.T) {
	// Given a Cybersecurity News article with content_encoded in source metadata.
	article := FeedArticle{SourceMetadata: map[string]json.RawMessage{
		"cybersecuritynews": json.RawMessage(`{"content_encoded":"<p>Full embedded story CVE-2026-9999</p>"}`),
	}}
	loader := NewArticleBodyLoader(nil, &browserBodyStub{})

	// When the article body is loaded.
	body, err := loader.Load(context.Background(), api.Source{Slug: "cybersecuritynews"}, article)

	// Then the embedded article is returned without a web request.
	if err != nil || body != "Full embedded story CVE-2026-9999" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestArticleBodyLoaderUsesOneHTTPRequest_whenSourceAllowsRequests(t *testing.T) {
	// Given an article page with a source-specific content container.
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = writer.Write([]byte(`<html><body><nav>Noise</nav><div id="news_content"><p>First paragraph.</p><p>Second paragraph.</p></div></body></html>`))
	}))
	defer upstream.Close()
	loader := NewArticleBodyLoader(upstream.Client(), &browserBodyStub{})

	// When the body is loaded over HTTP.
	body, err := loader.Load(context.Background(), api.Source{Slug: "boannews"}, FeedArticle{URL: upstream.URL})

	// Then exactly one request is sent and navigation noise is excluded.
	if err != nil || requests != 1 || body != "First paragraph.\n\nSecond paragraph." {
		t.Fatalf("requests = %d, body = %q, err = %v", requests, body, err)
	}
}

func TestArticleBodyLoaderFiltersStepSecurityProduct_whenHeaderBadgeIsProduct(t *testing.T) {
	// Given a StepSecurity page whose header badge identifies a Product post.
	markup := `<body><div class="page-wrapper main-padding with-nav-info-banner"><div class="main-wrapper"><div class="container-large padding-section-x-small"><article><div class="padding-section-large no-padding-top"><div class="blog-post-header grid-column-2"><div class="blog-post-header_left-column"><div class="margin-bottom"><div><a><div>Product</div></a></div></div></div></div><div class="blog-post-content_description"><p>Product announcement</p></div></div></article></div></div></div></body>`
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(markup))
	}))
	defer upstream.Close()
	loader := NewArticleBodyLoader(upstream.Client(), nil)

	// When the StepSecurity article body is loaded.
	_, err := loader.Load(context.Background(), api.Source{Slug: "stepsecurity"}, FeedArticle{URL: upstream.URL})

	// Then the Product post is rejected with the collector-visible filter signal.
	if !errors.Is(err, ErrArticleFiltered) {
		t.Fatalf("load error = %v, want ErrArticleFiltered", err)
	}
}

func TestCollectorSkipsFilteredStepSecurityProduct_withoutWarningOrPersistence(t *testing.T) {
	// Given an enabled StepSecurity source whose selected article is filtered as Product.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`UPDATE sources SET enabled = (slug = 'stepsecurity')`); err != nil {
		t.Fatalf("select StepSecurity source: %v", err)
	}
	collector := NewCollector(NewRepository(db), &allSourcesFeedStub{document: Document{
		Status: Status{OK: true}, Articles: []FeedArticle{{
			ID: "sha256:product", URL: "https://www.stepsecurity.io/blog/product", Title: "Product update",
			PublishedAt: "2026-08-03T01:00:00Z",
		}},
	}}, &filteredArticleBodyStub{})

	// When the selected day is collected.
	result, err := collector.Collect(context.Background(), "2026-08-03")

	// Then the Product post is silently excluded rather than saved as a failed article.
	if err != nil {
		t.Fatalf("collect day: %v", err)
	}
	if result.Collected != 0 || len(result.Warnings) != 0 {
		t.Fatalf("result = %+v", result)
	}
	var stored int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles WHERE feed_uid = 'sha256:product'`).Scan(&stored); err != nil {
		t.Fatalf("count product articles: %v", err)
	}
	if stored != 0 {
		t.Fatalf("stored product articles = %d, want 0", stored)
	}
}

func TestArticleBodyLoaderUsesChromiumOnly_whenSourceIsDarkReading(t *testing.T) {
	// Given a Dark Reading article and separate HTTP and Chromium loaders.
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer upstream.Close()
	browser := &browserBodyStub{body: "Browser-rendered article"}
	loader := NewArticleBodyLoader(upstream.Client(), browser)

	// When the protected article body is loaded.
	body, err := loader.Load(context.Background(), api.Source{Slug: "darkreading"}, FeedArticle{URL: upstream.URL})

	// Then Chromium is used once and no ordinary HTTP request is attempted.
	if err != nil || requests != 0 || browser.calls != 1 || body != "Browser-rendered article" {
		t.Fatalf("HTTP requests = %d, browser calls = %d, body = %q, err = %v", requests, browser.calls, body, err)
	}
}

func TestArticleBodyLoaderUsesChromiumOnly_whenSourceIsBleepingComputer(t *testing.T) {
	// Given a BleepingComputer article and separate HTTP and Chromium loaders.
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer upstream.Close()
	browser := &browserBodyStub{body: "Browser-rendered article"}
	loader := NewArticleBodyLoader(upstream.Client(), browser)

	// When the protected article body is loaded.
	body, err := loader.Load(context.Background(), api.Source{Slug: "bleepingcomputer"}, FeedArticle{URL: upstream.URL})

	// Then Chromium is used once and no ordinary HTTP request is attempted.
	if err != nil || requests != 0 || browser.calls != 1 || body != "Browser-rendered article" {
		t.Fatalf("HTTP requests = %d, browser calls = %d, body = %q, err = %v", requests, browser.calls, body, err)
	}
}

func TestCollectorStoresFullBodyAndExtractsCVE_whenCVEAppearsOnlyInArticleBody(t *testing.T) {
	// Given RSS metadata without a CVE and a crawled body that contains one.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	feedStub := &selectedFeedStub{document: Document{Status: Status{OK: true}, Articles: []FeedArticle{{
		ID: "sha256:body-cve", URL: "https://www.boannews.com/article", Title: "Incident report",
		PublishedAt: "2026-08-03T01:00:00Z", Description: "Short RSS description",
	}}}}
	collector := NewCollector(repository, feedStub, &articleBodyStub{body: "Full story details CVE-2026-48449."})

	// When the selected day is collected.
	_, err = collector.Collect(context.Background(), "2026-08-03")

	// Then the full body is stored and its CVE is linked to the article.
	if err != nil {
		t.Fatalf("collect day: %v", err)
	}
	var body string
	if err := db.QueryRow(`SELECT body FROM articles WHERE feed_uid = ?`, "sha256:body-cve").Scan(&body); err != nil {
		t.Fatalf("read article body: %v", err)
	}
	if body != "Full story details CVE-2026-48449." {
		t.Fatalf("body = %q", body)
	}
	cves, err := repository.CVEsForDay(context.Background(), "2026-08-03")
	if err != nil || len(cves) != 1 || cves[0] != "CVE-2026-48449" {
		t.Fatalf("CVEs = %v, err = %v", cves, err)
	}
}
