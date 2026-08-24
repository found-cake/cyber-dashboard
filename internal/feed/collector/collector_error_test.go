package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestHTTPFetcherReturnsResponseBody_whenUpstreamFails(t *testing.T) {
	// Given an RSS JSON endpoint that returns a detailed server error body.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`{"error":"feed generator failed","detail":"upstream timeout"}`))
	}))
	defer upstream.Close()
	fetcher := &HTTPFetcher{client: upstream.Client(), baseURL: upstream.URL}

	// When the source is fetched.
	_, err := fetcher.Fetch(context.Background(), api.Source{Slug: "broken"})

	// Then the returned error retains the complete response body for terminal logging.
	if err == nil || !strings.Contains(err.Error(), `{"error":"feed generator failed","detail":"upstream timeout"}`) {
		t.Fatalf("fetch error = %v, want complete response body", err)
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
