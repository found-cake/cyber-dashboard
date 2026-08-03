package feed

import "encoding/json"

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
