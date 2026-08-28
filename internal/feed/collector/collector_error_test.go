package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestHTTPFetcherReturnsBoundedSanitizedResponseBody_whenUpstreamFails(t *testing.T) {
	// Given an RSS JSON endpoint that returns an oversized error body containing a control character.
	const previewLimit = 4 << 10
	responseBody := "feed generator failed\n" + strings.Repeat("x", previewLimit) + "sensitive-tail"
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(responseBody))
	}))
	defer upstream.Close()
	fetcher := &HTTPFetcher{client: upstream.Client(), baseURL: upstream.URL}

	// When the source is fetched.
	_, err := fetcher.Fetch(context.Background(), api.Source{Slug: "broken"})

	// Then the returned error contains only a bounded, escaped preview.
	if err == nil {
		t.Fatal("fetch error = nil, want bounded response preview")
	}
	errorText := err.Error()
	if strings.Contains(errorText, "sensitive-tail") || strings.Contains(errorText, "\n") || !strings.Contains(errorText, `\n`) || !strings.Contains(errorText, "truncated") {
		t.Fatalf("fetch error = %q, want bounded, escaped, and marked response preview", errorText)
	}
}

func TestHTTPFetcherRejectsDocument_whenResponseExceedsLimit(t *testing.T) {
	// Given a valid RSS JSON document larger than the accepted feed response limit.
	oversizedSource := strings.Repeat("x", 8<<20)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"schema_version":1,"source":"` + oversizedSource + `","status":{"ok":true},"articles":[]}`))
	}))
	defer upstream.Close()
	fetcher := &HTTPFetcher{client: upstream.Client(), baseURL: upstream.URL}

	// When the source is fetched.
	_, err := fetcher.Fetch(context.Background(), api.Source{Slug: "oversized"})

	// Then the document is rejected before its full object graph is allocated.
	if err == nil || !strings.Contains(err.Error(), "feed response exceeds 8388608 bytes") {
		t.Fatalf("fetch error = %v, want response size rejection", err)
	}
}

func TestHTTPFetcherUsesPublishedSchema_whenFeedContainsLegacyCategory(t *testing.T) {
	// Given an RSS JSON document using a legacy category field supported by the published schema package.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"schema_version":1,"source":"cybersecuritynews","status":{"ok":true},"articles":[{"id":"article-1","url":"https://example.com/article","title":"Threat report","category":"Incidents","content_encoded":"<p>Full report</p>"}]}`))
	}))
	defer upstream.Close()
	fetcher := &HTTPFetcher{client: upstream.Client(), baseURL: upstream.URL}

	// When the source is fetched.
	document, err := fetcher.Fetch(context.Background(), api.Source{Slug: "cybersecuritynews"})

	// Then the schema package normalizes the legacy category into the current representation.
	if err != nil || len(document.Articles) != 1 || len(document.Articles[0].Categories) != 1 || document.Articles[0].Categories[0] != "Incidents" || document.Articles[0].EmbeddedContent != "<p>Full report</p>" {
		t.Fatalf("document = %+v, err = %v, want normalized legacy category", document, err)
	}
}

func TestHTTPFetcherRejectsDocument_whenSchemaVersionUnsupported(t *testing.T) {
	// Given an RSS JSON document with a schema version newer than the supported package contract.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"schema_version":2,"source":"future","status":{"ok":true},"articles":[]}`))
	}))
	defer upstream.Close()
	fetcher := &HTTPFetcher{client: upstream.Client(), baseURL: upstream.URL}

	// When the source is fetched.
	_, err := fetcher.Fetch(context.Background(), api.Source{Slug: "future"})

	// Then the unsupported contract is rejected before it enters the collector domain.
	if err == nil || !strings.Contains(err.Error(), "unsupported feed schema version 2") {
		t.Fatalf("fetch error = %v, want unsupported schema version", err)
	}
}

func TestHTTPFetcherRejectsDocument_whenDeclaredSourceDiffers(t *testing.T) {
	tests := []struct {
		name     string
		declared string
	}{
		{name: "different source", declared: `"dailysecu"`},
		{name: "missing source", declared: `""`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given a valid RSS document that does not declare the requested source.
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"schema_version":1,"source":` + test.declared + `,"status":{"ok":true},"articles":[]}`))
			}))
			defer upstream.Close()
			fetcher := &HTTPFetcher{client: upstream.Client(), baseURL: upstream.URL}

			// When the configured source is fetched.
			_, err := fetcher.Fetch(context.Background(), api.Source{Slug: "boannews"})

			// Then the feed is rejected before its articles can be attributed incorrectly.
			if err == nil || !strings.Contains(err.Error(), "source mismatch") {
				t.Fatalf("fetch error = %v, want source mismatch", err)
			}
		})
	}
}
