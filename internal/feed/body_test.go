package feed

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (s *selectedFeedStub) Fetch(_ context.Context, source api.Source) (Document, error) {
	if source.Slug == "bleepingcomputer" {
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

func (s *browserBodyStub) Load(_ context.Context, _, _ string) (string, error) {
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
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body: io.NopCloser(strings.NewReader(
				`<html><body><nav>Noise</nav><div id="news_content"><p>First paragraph.</p><p>Second paragraph.</p></div></body></html>`)),
			Request: request,
		}, nil
	})}
	loader := NewArticleBodyLoader(client, &browserBodyStub{})

	// When the body is loaded over HTTP.
	body, err := loader.Load(context.Background(), api.Source{Host: "boannews.com", Slug: "boannews"},
		FeedArticle{URL: "https://www.boannews.com/article"})

	// Then exactly one request is sent and navigation noise is excluded.
	if err != nil || requests != 1 || body != "First paragraph.\n\nSecond paragraph." {
		t.Fatalf("requests = %d, body = %q, err = %v", requests, body, err)
	}
}

func TestArticleBodyLoaderFiltersStepSecurityProduct_whenHeaderBadgeIsProduct(t *testing.T) {
	// Given a StepSecurity page whose header badge identifies a Product post.
	markup := `<body><div class="page-wrapper main-padding with-nav-info-banner"><div class="main-wrapper"><div class="container-large padding-section-x-small"><article><div class="padding-section-large no-padding-top"><div class="blog-post-header grid-column-2"><div class="blog-post-header_left-column"><div class="margin-bottom"><div><a><div>Product</div></a></div></div></div></div><div class="blog-post-content_description"><p>Product announcement</p></div></div></article></div></div></div></body>`
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader(markup)),
			Request:    request,
		}, nil
	})}
	loader := NewArticleBodyLoader(client, nil)

	// When the StepSecurity article body is loaded.
	_, err := loader.Load(context.Background(), api.Source{Host: "stepsecurity.io/blog", Slug: "stepsecurity"},
		FeedArticle{URL: "https://www.stepsecurity.io/blog/product"})

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
	t.Cleanup(func() { _ = database.Close(db) })
	if err := db.Exec(`UPDATE sources SET enabled = (slug = 'stepsecurity')`).Error; err != nil {
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
	if err := db.Raw(`SELECT COUNT(*) FROM articles WHERE feed_uid = 'sha256:product'`).Row().Scan(&stored); err != nil {
		t.Fatalf("count product articles: %v", err)
	}
	if stored != 0 {
		t.Fatalf("stored product articles = %d, want 0", stored)
	}
}

func TestArticleBodyLoaderUsesChromiumOnly_whenSourceIsDarkReading(t *testing.T) {
	// Given a Dark Reading article and separate HTTP and Chromium loaders.
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})}
	browser := &browserBodyStub{body: "Browser-rendered article"}
	loader := NewArticleBodyLoader(client, browser)

	// When the protected article body is loaded.
	body, err := loader.Load(context.Background(), api.Source{Host: "darkreading.com", Slug: "darkreading"},
		FeedArticle{URL: "https://www.darkreading.com/article"})

	// Then Chromium is used once and no ordinary HTTP request is attempted.
	if err != nil || requests != 0 || browser.calls != 1 || body != "Browser-rendered article" {
		t.Fatalf("HTTP requests = %d, browser calls = %d, body = %q, err = %v", requests, browser.calls, body, err)
	}
}

func TestArticleBodyLoaderUsesChromiumOnly_whenSourceIsBleepingComputer(t *testing.T) {
	// Given a BleepingComputer article and separate HTTP and Chromium loaders.
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})}
	browser := &browserBodyStub{body: "Browser-rendered article"}
	loader := NewArticleBodyLoader(client, browser)

	// When the protected article body is loaded.
	body, err := loader.Load(context.Background(), api.Source{Host: "bleepingcomputer.com", Slug: "bleepingcomputer"},
		FeedArticle{URL: "https://www.bleepingcomputer.com/news/security/article"})

	// Then Chromium is used once and no ordinary HTTP request is attempted.
	if err != nil || requests != 0 || browser.calls != 1 || body != "Browser-rendered article" {
		t.Fatalf("HTTP requests = %d, browser calls = %d, body = %q, err = %v", requests, browser.calls, body, err)
	}
}

func TestArticleBodyLoaderRejectsNonHTTPURL_beforeStartingChromium(t *testing.T) {
	// Given a protected source article with a local-file URL.
	browser := &browserBodyStub{body: "Local file contents"}
	loader := NewArticleBodyLoader(nil, browser)

	// When the article is sent to the browser boundary.
	_, err := loader.Load(context.Background(), api.Source{Host: "darkreading.com", Slug: "darkreading"},
		FeedArticle{URL: "file:///etc/passwd"})

	// Then the URL is rejected without invoking Chromium.
	if err == nil || browser.calls != 0 {
		t.Fatalf("browser calls = %d, error = %v, want rejection before Chromium", browser.calls, err)
	}
}

func TestArticleBodyLoaderRejectsURLOutsideConfiguredSourceHost_beforeStartingChromium(t *testing.T) {
	// Given a protected source whose article URL points at another public host.
	browser := &browserBodyStub{body: "Untrusted host contents"}
	loader := NewArticleBodyLoader(nil, browser)

	// When the mismatched article URL reaches the body-loading boundary.
	_, err := loader.Load(context.Background(), api.Source{Host: "darkreading.com", Slug: "darkreading"},
		FeedArticle{URL: "https://example.com/article"})

	// Then the URL is rejected without invoking Chromium.
	if err == nil || browser.calls != 0 {
		t.Fatalf("browser calls = %d, error = %v, want source-host rejection", browser.calls, err)
	}
}

func TestArticleBodyLoaderRejectsPrivateNetworkURL_beforeStartingChromium(t *testing.T) {
	// Given a protected source configured with a loopback article host.
	browser := &browserBodyStub{body: "Private service contents"}
	loader := NewArticleBodyLoader(nil, browser)

	// When the private-network article URL reaches the body-loading boundary.
	_, err := loader.Load(context.Background(), api.Source{Host: "127.0.0.1", Slug: "darkreading"},
		FeedArticle{URL: "http://127.0.0.1/admin"})

	// Then the URL is rejected without invoking Chromium.
	if err == nil || browser.calls != 0 {
		t.Fatalf("browser calls = %d, error = %v, want private-network rejection", browser.calls, err)
	}
}

func TestArticleBodyLoaderRejectsCrossHostHTTPRedirect_beforeFollowingIt(t *testing.T) {
	// Given a source response that redirects its article request to loopback.
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://127.0.0.1/admin"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<article>Private service contents</article>")),
			Request:    request,
		}, nil
	})}
	loader := NewArticleBodyLoader(client, nil)

	// When an ordinary HTTP source article is loaded.
	_, err := loader.Load(context.Background(), api.Source{Host: "boannews.com", Slug: "boannews"},
		FeedArticle{URL: "https://www.boannews.com/article"})

	// Then redirect validation stops the request before loopback is contacted.
	if err == nil || requests != 1 {
		t.Fatalf("requests = %d, error = %v, want one request and redirect rejection", requests, err)
	}
}

func TestCollectorStoresFullBodyAndExtractsCVE_whenCVEAppearsOnlyInArticleBody(t *testing.T) {
	// Given RSS metadata without a CVE and a crawled body that contains one.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
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
	if err := db.Raw(`SELECT body FROM articles WHERE feed_uid = ?`, "sha256:body-cve").Row().Scan(&body); err != nil {
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
