//go:build browser

package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChromiumBodyLoaderReusesBrowserSession_whenLoadingMultipleArticles(t *testing.T) {
	// Given two articles where the second requires the clearance cookie issued by the first.
	articleText := strings.Repeat("session article body ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if request.URL.Path == "/first" {
			http.SetCookie(writer, &http.Cookie{Name: "cf_clearance", Value: "qa", Path: "/"})
			_, _ = fmt.Fprintf(writer, "<article>%s</article>", articleText)
			return
		}
		cookie, err := request.Cookie("cf_clearance")
		if err != nil || cookie.Value != "qa" {
			_, _ = writer.Write([]byte("<main>Performing security verification</main>"))
			return
		}
		_, _ = fmt.Fprintf(writer, "<article>%s</article>", articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When the same loader retrieves both articles in sequence.
	if _, err := loader.Load(ctx, server.URL+"/first", server.URL); err != nil {
		t.Fatalf("load first article: %v", err)
	}
	body, err := loader.Load(ctx, server.URL+"/second", server.URL)

	// Then the second tab shares the first tab's browser session and cookie jar.
	if err != nil {
		t.Fatalf("load second article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want second article text", len(body))
	}
}

func TestChromiumBodyLoaderReusesPageSession_whenLoadingMultipleArticles(t *testing.T) {
	// Given two same-origin pages where the second requires tab-scoped session storage from the first.
	articleText := strings.Repeat("page session article body ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if request.URL.Path == "/first" {
			_, _ = fmt.Fprintf(writer, `<article>%s</article><script>sessionStorage.setItem("clearance", "qa")</script>`, articleText)
			return
		}
		_, _ = fmt.Fprintf(writer, `<body><script>
document.body.innerHTML = sessionStorage.getItem("clearance") === "qa"
  ? %q
  : "<main>Performing security verification</main>";
</script></body>`, "<article>"+articleText+"</article>")
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When the same loader navigates from the first article to the second.
	if _, err := loader.Load(ctx, server.URL+"/first", server.URL); err != nil {
		t.Fatalf("load first article: %v", err)
	}
	body, err := loader.Load(ctx, server.URL+"/second", server.URL)

	// Then the second navigation retains the first page's tab-scoped state.
	if err != nil {
		t.Fatalf("load second article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want second article text", len(body))
	}
}
