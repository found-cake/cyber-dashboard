package collector

import (
	"fmt"

	"github.com/found-cake/cyber-news-feed/pkg/rssjson"
)

func documentFromRSSJSON(wire rssjson.Document) (Document, error) {
	if wire.SchemaVersion != rssjson.SchemaVersion {
		return Document{}, fmt.Errorf("unsupported feed schema version %d", wire.SchemaVersion)
	}
	articles := make([]FeedArticle, len(wire.Articles))
	for index, article := range wire.Articles {
		publishedAt := ""
		if article.PublishedAt != nil {
			publishedAt = *article.PublishedAt
		}
		embeddedContent := ""
		if metadata, ok := article.SourceMetadata.Object("cybersecuritynews"); ok {
			embeddedContent, _ = metadata.Text("content_encoded")
		}
		articles[index] = FeedArticle{
			ID:              article.ID,
			URL:             article.URL,
			Title:           article.Title,
			PublishedAt:     publishedAt,
			PublishedRaw:    article.PublishedRaw,
			Description:     article.Description,
			Categories:      article.Categories,
			EmbeddedContent: embeddedContent,
		}
	}
	return Document{Status: Status{OK: wire.Status.OK}, Articles: articles}, nil
}
