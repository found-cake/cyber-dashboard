package collector

import (
	"encoding/json"
	"net/mail"
	"strings"
	"time"
)

type Document struct {
	Status   Status        `json:"status"`
	Articles []FeedArticle `json:"articles"`
}

type Status struct {
	OK bool `json:"ok"`
}

type FeedArticle struct {
	ID             string                     `json:"id"`
	URL            string                     `json:"url"`
	Title          string                     `json:"title"`
	PublishedAt    string                     `json:"published_at"`
	PublishedRaw   string                     `json:"published_raw"`
	Description    string                     `json:"description"`
	Body           string                     `json:"-"`
	Categories     []string                   `json:"categories"`
	SourceMetadata map[string]json.RawMessage `json:"source_metadata"`
}

func (article FeedArticle) PublishedTime() (time.Time, bool) {
	if article.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, article.PublishedAt); err == nil {
			return parsed.UTC(), true
		}
	}
	if strings.TrimSpace(article.PublishedRaw) != "" {
		if parsed, err := mail.ParseDate(article.PublishedRaw); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
