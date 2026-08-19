package integrationtest

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	web "github.com/found-cake/cyber-dashboard/internal/web/httpapi"
)

func TestStaticCacheControl(t *testing.T) {
	// Given a real router serving both cacheable assets and revalidated documents.
	assets := fs.FS(fstest.MapFS{
		"app.js":        {Data: []byte("console.log('ok')")},
		"index.html":    {Data: []byte("index")},
		"showcase.html": {Data: []byte("showcase")},
		"styles.css":    {Data: []byte("body {}")},
	})
	server := web.NewServer(web.Dependencies{Assets: assets, AllowUntrustedHosts: true})
	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantCacheCtl string
	}{
		{name: "root document", path: "/", wantStatus: http.StatusOK, wantCacheCtl: "no-cache"},
		{name: "named document", path: "/showcase.html", wantStatus: http.StatusOK, wantCacheCtl: "no-cache"},
		{name: "application script", path: "/app.js", wantStatus: http.StatusOK, wantCacheCtl: "no-cache"},
		{name: "stylesheet asset", path: "/styles.css", wantStatus: http.StatusOK, wantCacheCtl: "public, max-age=3600"},
		{name: "health endpoint", path: "/healthz", wantStatus: http.StatusOK},
		{name: "missing asset", path: "/missing.js", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When the resource is requested through the production route stack.
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			// Then only successful static responses expose the matching browser cache policy.
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if got := response.Header().Get("Cache-Control"); got != test.wantCacheCtl {
				t.Fatalf("Cache-Control = %q, want %q", got, test.wantCacheCtl)
			}
		})
	}
}
