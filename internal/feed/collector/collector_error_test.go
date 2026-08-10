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
