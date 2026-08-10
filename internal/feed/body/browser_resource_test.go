//go:build browser

package body

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestChromiumBodyLoaderBoundsNavigationHistory_whenLoadingMultipleArticles(t *testing.T) {
	// Given a shared Chromium page and two article URLs.
	articleText := strings.Repeat("security article details ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, "<article>%s</article>", articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When the shared page loads multiple articles.
	if _, err := loader.Load(ctx, server.URL+"/first", server.URL); err != nil {
		t.Fatalf("load first article: %v", err)
	}
	if _, err := loader.Load(ctx, server.URL+"/second", server.URL); err != nil {
		t.Fatalf("load second article: %v", err)
	}
	var historyLength int
	if err := chromedp.Run(loader.context, chromedp.Evaluate(`history.length`, &historyLength)); err != nil {
		t.Fatalf("read navigation history: %v", err)
	}

	// Then only the current entry remains eligible for renderer retention.
	if historyLength != 1 {
		t.Fatalf("navigation history length = %d, want 1", historyLength)
	}
}

func TestChromiumBodyLoaderInjectsDocumentScriptOnce_whenLoadingMultipleArticles(t *testing.T) {
	// Given a shared Chromium page and several article URLs.
	articleText := strings.Repeat("injected script article ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, "<article>%s</article>", articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When the shared page loads several articles in sequence.
	for index := 0; index < 3; index++ {
		if _, err := loader.Load(ctx, fmt.Sprintf("%s/article%d", server.URL, index), server.URL); err != nil {
			t.Fatalf("load article %d: %v", index, err)
		}
	}

	// Then the new-document script is injected once, not once per article: the injection
	// persists on the page, so stacking copies would re-run them on every navigation.
	if loader.documentScriptInjections != 1 {
		t.Fatalf("document script injections = %d, want 1", loader.documentScriptInjections)
	}
	var webdriver any
	if err := chromedp.Run(loader.context, chromedp.Evaluate(`navigator.webdriver`, &webdriver)); err != nil {
		t.Fatalf("read navigator.webdriver: %v", err)
	}
	if webdriver != nil {
		t.Fatalf("navigator.webdriver = %v, want undefined", webdriver)
	}
}

func TestChromiumBodyLoaderDropsImageBytes_whileStillExtractingTheArticle(t *testing.T) {
	// Given an article page carrying images, a font, and a stylesheet that styles the body.
	articleText := strings.Repeat("image heavy article body ", 30)
	var imageRequests, fontRequests, styleRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, ".png"):
			imageRequests.Add(1)
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write(make([]byte, 256*1024))
		case strings.HasSuffix(request.URL.Path, ".woff2"):
			fontRequests.Add(1)
			writer.Header().Set("Content-Type", "font/woff2")
			_, _ = writer.Write(make([]byte, 64*1024))
		case strings.HasSuffix(request.URL.Path, ".css"):
			styleRequests.Add(1)
			writer.Header().Set("Content-Type", "text/css")
			_, _ = writer.Write([]byte(".promo{display:none}"))
		default:
			_, _ = fmt.Fprintf(writer, `<html><head><link rel="stylesheet" href="/a.css"></head>
<body><img src="/one.png"><img src="/two.png"><img src="/three.png">
<span class="promo">HIDDEN PROMO TEXT</span><article>%s</article></body></html>`, articleText)
		}
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When the article is loaded.
	body, err := loader.Load(ctx, server.URL+"/article", server.URL)

	// Then the article text is complete and the image and font bytes were never fetched,
	// while the stylesheet still loads so hidden content stays out of innerText.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body = %q, want article text only", body)
	}
	if imageRequests.Load() != 0 {
		t.Fatalf("image requests = %d, want 0", imageRequests.Load())
	}
	if fontRequests.Load() != 0 {
		t.Fatalf("font requests = %d, want 0", fontRequests.Load())
	}
	if styleRequests.Load() == 0 {
		t.Fatal("stylesheet was not requested, so hidden content could leak into innerText")
	}
}

func TestChromiumBodyLoaderIncludesPageState_whenCallerDeadlineExpires(t *testing.T) {
	// Given an article whose body never reaches the extraction threshold.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("<article>short body</article>"))
	}))
	t.Cleanup(server.Close)
	parentContext, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(parentCancel)
	loader := NewChromiumBodyLoader(parentContext)
	t.Cleanup(loader.Close)
	loadContext, loadCancel := context.WithTimeout(parentContext, 300*time.Millisecond)
	defer loadCancel()

	// When the caller deadline expires during body polling.
	_, err := loader.Load(loadContext, server.URL, server.URL)

	// Then the error contains diagnostic page state instead of empty parentheses.
	if err == nil || strings.HasSuffix(strings.TrimSpace(err.Error()), "()") {
		t.Fatalf("load error = %v, want page-state diagnostics", err)
	}
}

func TestChromiumBodyLoaderReturnsArticle_beforeSlowSubresourceFinishes(t *testing.T) {
	// Given an article whose main DOM is ready while an unrelated stylesheet request remains
	// pending. A stylesheet stands in for any slow subresource the loader still fetches;
	// images are dropped before they reach the network.
	articleText := strings.Repeat("security article details ", 30)
	slowRequest := make(chan struct{}, 1)
	slowRequestCancelled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/slow-style.css" {
			slowRequest <- struct{}{}
			<-request.Context().Done()
			slowRequestCancelled <- struct{}{}
			return
		}
		_, _ = fmt.Fprintf(writer, `<html><head><link rel="stylesheet" href="/slow-style.css"></head><body><article>%s</article></body></html>`, articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium extracts the article without waiting for the unrelated resource.
	body, err := loader.Load(ctx, server.URL, server.URL)

	// Then the visible article is returned before the caller deadline expires.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	select {
	case <-slowRequest:
	default:
		t.Fatal("slow subresource was not requested")
	}
	cancellationContext, cancellationCancel := context.WithTimeout(context.Background(), time.Second)
	defer cancellationCancel()
	select {
	case <-slowRequestCancelled:
	case <-cancellationContext.Done():
		t.Fatal("slow subresource remained active after article extraction")
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want complete article text", len(body))
	}
}

func TestChromiumBodyLoaderAllowsCrossHostSubresource_whenMainDocumentHostMatches(t *testing.T) {
	// Given a source-owned article whose stylesheet is served by another host authority.
	// The subresource is a stylesheet rather than an image so it exercises the host policy
	// instead of the resource-type filter that drops images.
	assetRequests := make(chan struct{}, 1)
	assets := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		assetRequests <- struct{}{}
		writer.Header().Set("Content-Type", "text/css")
		_, _ = writer.Write([]byte("article{color:#111}"))
	}))
	t.Cleanup(assets.Close)
	articleText := strings.Repeat("cross-host asset article ", 30)
	article := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, `<html><head><link rel="stylesheet" href="%s/style.css"></head><body><article>%s</article></body></html>`, assets.URL, articleText)
	}))
	t.Cleanup(article.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium loads the source-owned main document.
	body, err := loader.Load(ctx, article.URL, article.URL)

	// Then the article and its cross-host subresource both load successfully.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want complete article text", len(body))
	}
	select {
	case <-assetRequests:
	case <-ctx.Done():
		t.Fatal("cross-host subresource was not requested")
	}
}

func TestChromiumBodyLoaderRecreatesBrowserSession_afterConnectionLoss(t *testing.T) {
	// Given a shared Chromium session that has completed one article load.
	articleText := strings.Repeat("recovered browser article ", 30)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(writer, "<article>%s</article>", articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)
	if _, err := loader.Load(ctx, server.URL+"/first", server.URL); err != nil {
		t.Fatalf("load first article: %v", err)
	}
	firstBrowser := chromedp.FromContext(loader.context).Browser
	firstProcess := firstBrowser.Process()
	if firstProcess == nil {
		t.Fatal("first Chromium process is unavailable")
	}

	// When the browser connection is lost before the next article load.
	if err := firstProcess.Kill(); err != nil {
		t.Fatalf("terminate first Chromium process: %v", err)
	}
	select {
	case <-firstBrowser.LostConnection:
	case <-ctx.Done():
		t.Fatal("first Chromium connection did not close")
	}
	body, err := loader.Load(ctx, server.URL+"/second", server.URL)

	// Then the loader creates a new session and completes the next load.
	if err != nil {
		t.Fatalf("load second article: %v", err)
	}
	secondBrowser := chromedp.FromContext(loader.context).Browser
	if secondBrowser == firstBrowser {
		t.Fatal("loader reused the disconnected Chromium session")
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body = %q, want recovered article text", body)
	}
}
