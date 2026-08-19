package integrationtest

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/web"
)

func TestDashboardCompressionHonorsGzipQuality_whenClientNegotiatesEncoding(t *testing.T) {
	tests := []struct {
		name         string
		accept       string
		wantEncoding string
	}{
		{name: "accepts gzip", accept: "gzip", wantEncoding: "gzip"},
		{name: "accepts mixed case gzip", accept: "GZip;Q=0.5", wantEncoding: "gzip"},
		{name: "accepts gzip among other encodings", accept: "br, gzip; q=0.5", wantEncoding: "gzip"},
		{name: "accepts wildcard", accept: "br, *;q=0.5", wantEncoding: "gzip"},
		{name: "explicit gzip overrides rejected wildcard", accept: "gzip;q=0.5, *;q=0", wantEncoding: "gzip"},
		{name: "rejects gzip with zero quality", accept: "gzip;q=0"},
		{name: "specific rejection overrides wildcard", accept: "gzip;q=0, *;q=1"},
		{name: "rejects wildcard with zero quality", accept: "*;q=0"},
		{name: "rejects malformed gzip quality", accept: "gzip;q=invalid, *;q=1"},
		{name: "rejects out of range gzip quality", accept: "gzip;q=2, *;q=1"},
		{name: "does not negotiate an absent encoding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given a dashboard response large enough for API compression.
			server, _, _ := newTestServer(t, &stubFetcher{})
			request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
			request.Header.Set("Accept-Encoding", test.accept)
			recorder := httptest.NewRecorder()

			// When the client sends its Accept-Encoding preferences.
			server.ServeHTTP(recorder, request)

			// Then gzip is selected only when the header allows it.
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Header().Get("Content-Encoding"); got != test.wantEncoding {
				t.Fatalf("content encoding = %q, want %q", got, test.wantEncoding)
			}
			if vary := strings.ToLower(strings.Join(recorder.Header().Values("Vary"), ",")); !strings.Contains(vary, "accept-encoding") {
				t.Fatalf("Vary = %q, want Accept-Encoding", vary)
			}
			var body io.Reader = recorder.Body
			if test.wantEncoding == "gzip" {
				compressed, err := gzip.NewReader(recorder.Body)
				if err != nil {
					t.Fatalf("open gzip response: %v", err)
				}
				defer compressed.Close()
				body = compressed
			}
			payload, err := io.ReadAll(body)
			if err != nil {
				t.Fatalf("read dashboard response: %v", err)
			}
			var dashboard api.Dashboard
			if err := json.Unmarshal(payload, &dashboard); err != nil {
				t.Fatalf("decode dashboard response: %v", err)
			}
			if !dashboard.Empty || len(dashboard.Trend) != 10 {
				t.Fatalf("dashboard response = empty %t, trend %d", dashboard.Empty, len(dashboard.Trend))
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", contentType)
			}
		})
	}
}

func TestDashboardCompressionSkipsSmallAPIResponsesAndStaticAssets(t *testing.T) {
	// Given a small API response and a static asset larger than the gzip threshold.
	server, _, _ := newTestServer(t, &stubFetcher{})
	smallRequest := httptest.NewRequest(http.MethodGet, "/api/cves", nil)
	smallRequest.Header.Set("Accept-Encoding", "gzip")
	smallRecorder := httptest.NewRecorder()
	server.ServeHTTP(smallRecorder, smallRequest)

	staticBody := strings.Repeat("s", gzipMinimumLengthForTest+1)
	staticServer := web.NewServer(web.Dependencies{
		Assets: fstest.MapFS{"index.html": {Data: []byte(staticBody)}}, AllowUntrustedHosts: true,
	})
	staticRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	staticRequest.Header.Set("Accept-Encoding", "gzip")
	staticRecorder := httptest.NewRecorder()

	// When the large static asset is served through the same middleware stack.
	staticServer.ServeHTTP(staticRecorder, staticRequest)

	// Then neither the sub-threshold API response nor the static asset is compressed.
	if smallRecorder.Code != http.StatusOK || smallRecorder.Body.Len() >= gzipMinimumLengthForTest {
		t.Fatalf("small API response = status %d, %d bytes", smallRecorder.Code, smallRecorder.Body.Len())
	}
	if got := smallRecorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("small API content encoding = %q, want none", got)
	}
	if staticRecorder.Code != http.StatusOK || staticRecorder.Body.String() != staticBody {
		t.Fatalf("static response = status %d, %d bytes", staticRecorder.Code, staticRecorder.Body.Len())
	}
	if got := staticRecorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("static content encoding = %q, want none", got)
	}
}

const gzipMinimumLengthForTest = 1024
