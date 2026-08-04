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

func TestChromiumBodyLoaderReturnsArticle_whenArticleTextCanOnlyBeReadOnce(t *testing.T) {
	// Given an article whose text becomes unavailable after the first DOM observation.
	articleText := strings.Repeat("article body ", 40)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, `<html><body><article>placeholder</article><script>
let reads = 0;
const value = %q;
const article = document.querySelector("article");
Object.defineProperty(article, "innerText", {get: () => {
  reads += 1;
  if (reads > 1) {
    queueMicrotask(() => document.body.remove());
    return "";
  }
  return value;
}});
</script></body></html>`, articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium waits for and extracts the article.
	body, err := loader.Load(ctx, server.URL, server.URL)

	// Then extraction returns the complete article from that single observation.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want complete article text", len(body))
	}
}

func TestChromiumBodyLoaderReturnsBleepingComputerArticle_whenAdvertisementUsesGenericClass(t *testing.T) {
	// Given a BleepingComputer-shaped page where an advertisement uses the generic article-body class.
	advertisementText := strings.Repeat("advertisement copy ", 30)
	articleText := strings.Repeat("security article details ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, `<div class="article-body">%s</div><article><div class="articleBody">%s</div></article>`, advertisementText, articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium extracts the page body.
	body, err := loader.Load(ctx, server.URL, server.URL)

	// Then the BleepingComputer article is returned instead of the advertisement.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body = %q, want BleepingComputer article", body)
	}
}

func TestChromiumBodyLoaderReturnsArticle_whenChallengeNavigatesToArticle(t *testing.T) {
	// Given a challenge page that performs a client-side navigation to the real article.
	articleText := strings.Repeat("navigated article body ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if request.URL.Path == "/article" {
			_, _ = fmt.Fprintf(writer, "<article>%s</article>", articleText)
			return
		}
		_, _ = writer.Write([]byte(`<main>Verifying browser</main><script>setTimeout(() => location.href = "/article", 100)</script>`))
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium waits through the challenge navigation.
	body, err := loader.Load(ctx, server.URL, server.URL)

	// Then extraction continues on the new document and returns the article.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want navigated article text", len(body))
	}
}

func TestChromiumBodyLoaderRejectsCrossHostDocumentRedirect_beforeLoadingDestination(t *testing.T) {
	// Given an article URL that redirects the main document to another host authority.
	destinationRequests := make(chan struct{}, 1)
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		destinationRequests <- struct{}{}
		_, _ = fmt.Fprintf(writer, "<article>%s</article>", strings.Repeat("redirected article ", 30))
	}))
	t.Cleanup(destination.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL+"/article", http.StatusFound)
	}))
	t.Cleanup(origin.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium follows the initial navigation.
	_, err := loader.Load(ctx, origin.URL, origin.URL)

	// Then the cross-host main-document request is blocked before reaching its server.
	if err == nil {
		t.Fatal("cross-host document redirect succeeded, want policy rejection")
	}
	select {
	case <-destinationRequests:
		t.Fatal("cross-host redirect destination received a request")
	default:
	}
}
