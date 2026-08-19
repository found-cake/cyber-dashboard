//go:build browser

package browsertest

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/chromedp/chromedp"
	web "github.com/found-cake/cyber-dashboard/internal/web/httpapi"
)

func TestStaticCachePolicyInBrowser(t *testing.T) {
	// Given two HTML documents that load the same script and stylesheet through the production server.
	assets := fs.FS(fstest.MapFS{
		"app.js":        {Data: []byte("window.cachedScriptLoaded = true")},
		"index.html":    {Data: []byte(`<link rel="stylesheet" href="/styles.css"><script src="/app.js"></script>`)},
		"showcase.html": {Data: []byte(`<link rel="stylesheet" href="/styles.css"><script src="/app.js"></script>`)},
		"styles.css":    {Data: []byte("body { color: black }")},
	})
	productionServer := web.NewServer(web.Dependencies{Assets: assets, AllowUntrustedHosts: true})
	var scriptRequests atomic.Int64
	var stylesheetRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/app.js":
			scriptRequests.Add(1)
		case "/styles.css":
			stylesheetRequests.Add(1)
		}
		productionServer.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)
	browser := newBrowserContext(t, 15*time.Second)

	// When Chrome visits both documents in the same browser context.
	for _, path := range []string{"/", "/showcase.html"} {
		var scriptLoaded bool
		if err := chromedp.Run(browser,
			chromedp.Navigate(server.URL+path),
			chromedp.Evaluate(`window.cachedScriptLoaded === true`, &scriptLoaded),
		); err != nil {
			t.Fatalf("navigate to %s: %v", path, err)
		}
		if !scriptLoaded {
			t.Fatalf("script did not load from %s", path)
		}
	}

	// Then Chrome revalidates app.js while reusing the fresh stylesheet.
	if got := scriptRequests.Load(); got != 2 {
		t.Fatalf("app.js requests = %d, want 2", got)
	}
	if got := stylesheetRequests.Load(); got != 1 {
		t.Fatalf("styles.css requests = %d, want 1", got)
	}
}
