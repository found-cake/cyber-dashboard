package body

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
)

const maximumArticlePageBytes = 8 << 20

type BrowserBodyLoader interface {
	Load(ctx context.Context, request BrowserLoadRequest) (string, error)
}

type BrowserLoadRequest struct {
	ArticleURL string
	SourceHost string
	SourceSlug string
}

type ArticleBodyLoader struct {
	client  *http.Client
	browser BrowserBodyLoader
}

func NewArticleBodyLoader(client *http.Client, browser BrowserBodyLoader) *ArticleBodyLoader {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &ArticleBodyLoader{client: client, browser: browser}
}

func (l *ArticleBodyLoader) Load(ctx context.Context, source api.Source, article collector.FeedArticle) (string, error) {
	if usesRSSMetadataOnly(source.Slug) {
		return "", nil
	}
	if source.Slug == "cybersecuritynews" {
		return embeddedArticleBody(article)
	}
	parsed, err := url.ParseRequestURI(article.URL)
	if err != nil {
		return "", fmt.Errorf("invalid article URL: %w", err)
	}
	policy, err := newArticleURLPolicy(source.Host)
	if err != nil {
		return "", fmt.Errorf("invalid source host: %w", err)
	}
	if err := policy.validate(parsed, true); err != nil {
		return "", fmt.Errorf("invalid article URL: %w", err)
	}
	target := validatedArticleURL{url: parsed, policy: policy}
	info := articleContentSelectors[source.Slug]
	if info.needBrowser {
		if l.browser == nil {
			return "", fmt.Errorf("Chromium is unavailable for %s", source.Slug)
		}
		return l.browser.Load(ctx, BrowserLoadRequest{
			ArticleURL: target.url.String(),
			SourceHost: target.policy.authority(),
			SourceSlug: source.Slug,
		})
	}
	return l.loadHTTP(ctx, source.Slug, target)
}

func embeddedArticleBody(article collector.FeedArticle) (string, error) {
	if article.EmbeddedContent == "" {
		return "", fmt.Errorf("Cybersecurity News content is missing")
	}
	return extractArticleText(article.EmbeddedContent, "")
}
