package body

import (
	"context"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
)

func TestArticleBodyLoaderUsesEmbeddedContent_whenFeedContainsFullArticle(t *testing.T) {
	// Given a Cybersecurity News article with content_encoded in source metadata.
	article := collector.FeedArticle{EmbeddedContent: "<p>Full embedded story CVE-2026-9999</p>"}
	loader := NewArticleBodyLoader(nil, nil)

	// When the article body is loaded.
	body, err := loader.Load(context.Background(), api.Source{Slug: "cybersecuritynews"}, article)

	// Then the embedded article is returned without a web request.
	if err != nil || body != "Full embedded story CVE-2026-9999" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestArticleBodyLoaderUsesRSSMetadataOnly_whenSourceDisallowsArticleRequests(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{name: "BoanNews", slug: "boannews"},
		{name: "DailySecu", slug: "dailysecu"},
		{name: "StepSecurity", slug: "stepsecurity"},
		{name: "Dark Reading", slug: "darkreading"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given an RSS-only source whose article URL cannot be loaded.
			loader := NewArticleBodyLoader(nil, nil)

			// When the collector asks for an article body.
			body, err := loader.Load(context.Background(), api.Source{Slug: test.slug},
				collector.FeedArticle{URL: "://invalid"})

			// Then the invalid page URL is ignored and only RSS metadata is used.
			if err != nil || body != "" {
				t.Fatalf("body = %q, err = %v, want RSS-only collection", body, err)
			}
		})
	}
}
