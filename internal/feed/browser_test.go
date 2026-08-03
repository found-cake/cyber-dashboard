//go:build browser

package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestChromiumBodyLoaderSendsNormalChromeUserAgent_whenNavigatingArticle(t *testing.T) {
	// Given a real Chromium session and an article server that records its first request.
	type browserIdentity struct {
		userAgent   string
		clientHints string
	}
	received := make(chan browserIdentity, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case received <- browserIdentity{userAgent: request.UserAgent(), clientHints: request.Header.Get("Sec-Ch-Ua")}:
		default:
		}
		_, _ = fmt.Fprintf(writer, "<article><p>%s</p></article>", strings.Repeat("article body ", 40))
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium navigates to the article.
	if _, err := loader.Load(ctx, server.URL); err != nil {
		t.Fatalf("load article: %v", err)
	}

	// Then the HTTP boundary receives the requested non-headless Chrome identity.
	select {
	case got := <-received:
		if strings.Contains(got.userAgent, "HeadlessChrome") || !strings.Contains(got.userAgent, " Chrome/") {
			t.Fatalf("User-Agent = %q, want normal Chrome identity", got.userAgent)
		}
		version := strings.Split(strings.SplitN(got.userAgent, " Chrome/", 2)[1], ".")[0]
		if !strings.Contains(got.clientHints, `"Chromium";v="`+version+`"`) || !strings.Contains(got.clientHints, `"Google Chrome";v="`+version+`"`) {
			t.Fatalf("Sec-CH-UA = %q, want Chrome %s brands", got.clientHints, version)
		}
	case <-ctx.Done():
		t.Fatal("article server did not receive a Chromium request")
	}
}

func TestChromiumBodyLoaderSendsProcessArchitecture_whenServerRequestsClientHint(t *testing.T) {
	// Given an origin that requests the high-entropy architecture client hint before serving an article.
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			writer.Header().Set("Accept-CH", "Sec-CH-UA-Arch")
			_, _ = writer.Write([]byte(`<script>location.href = "/article"</script>`))
			return
		}
		received <- request.Header.Get("Sec-CH-UA-Arch")
		_, _ = fmt.Fprintf(writer, "<article><p>%s</p></article>", strings.Repeat("article body ", 40))
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium follows the navigation after accepting the client hint.
	if _, err := loader.Load(ctx, server.URL); err != nil {
		t.Fatalf("load article: %v", err)
	}

	// Then the declared architecture matches the running process instead of a hard-coded value.
	want := `"x86"`
	if runtime.GOARCH == "arm64" {
		want = `"arm"`
	}
	select {
	case got := <-received:
		if got != want {
			t.Fatalf("Sec-CH-UA-Arch = %q, want %q for %s", got, want, runtime.GOARCH)
		}
	case <-ctx.Done():
		t.Fatal("article server did not receive the architecture client hint")
	}
}

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
	body, err := loader.Load(ctx, server.URL)

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
	body, err := loader.Load(ctx, server.URL)

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
	body, err := loader.Load(ctx, server.URL)

	// Then extraction continues on the new document and returns the article.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want navigated article text", len(body))
	}
}

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
	if _, err := loader.Load(ctx, server.URL+"/first"); err != nil {
		t.Fatalf("load first article: %v", err)
	}
	body, err := loader.Load(ctx, server.URL+"/second")

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
	if _, err := loader.Load(ctx, server.URL+"/first"); err != nil {
		t.Fatalf("load first article: %v", err)
	}
	body, err := loader.Load(ctx, server.URL+"/second")

	// Then the second navigation retains the first page's tab-scoped state.
	if err != nil {
		t.Fatalf("load second article: %v", err)
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want second article text", len(body))
	}
}

func TestChromiumBodyLoaderReturnsArticle_beforeSlowSubresourceFinishes(t *testing.T) {
	// Given an article whose main DOM is ready while an unrelated image request remains pending.
	articleText := strings.Repeat("security article details ", 30)
	slowRequest := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/slow-image" {
			slowRequest <- struct{}{}
			<-request.Context().Done()
			return
		}
		_, _ = fmt.Fprintf(writer, `<html><body><img src="/slow-image"><article>%s</article></body></html>`, articleText)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	loader := NewChromiumBodyLoader(ctx)
	t.Cleanup(loader.Close)

	// When Chromium extracts the article without waiting for the unrelated resource.
	body, err := loader.Load(ctx, server.URL)

	// Then the visible article is returned before the caller deadline expires.
	if err != nil {
		t.Fatalf("load article: %v", err)
	}
	select {
	case <-slowRequest:
	default:
		t.Fatal("slow subresource was not requested")
	}
	if strings.TrimSpace(body) != strings.TrimSpace(articleText) {
		t.Fatalf("body length = %d, want complete article text", len(body))
	}
}
