package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

const maximumArticlePageBytes = 8 << 20

var ErrArticleFiltered = errors.New("article filtered by source policy")

type BrowserBodyLoader interface {
	Load(ctx context.Context, articleURL, sourceHost string) (string, error)
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

func (l *ArticleBodyLoader) Load(ctx context.Context, source api.Source, article FeedArticle) (string, error) {
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
	switch source.Slug {
	case "darkreading", "bleepingcomputer":
		if l.browser == nil {
			return "", fmt.Errorf("Chromium is unavailable for %s", source.Slug)
		}
		return l.browser.Load(ctx, target.url.String(), target.policy.authority())
	}
	return l.loadHTTP(ctx, source.Slug, target)
}

func embeddedArticleBody(article FeedArticle) (string, error) {
	raw, ok := article.SourceMetadata["cybersecuritynews"]
	if !ok {
		return "", fmt.Errorf("Cybersecurity News content is missing")
	}
	var metadata struct {
		Content string `json:"content_encoded"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "", fmt.Errorf("decode Cybersecurity News content: %w", err)
	}
	return extractArticleText(metadata.Content, "")
}
