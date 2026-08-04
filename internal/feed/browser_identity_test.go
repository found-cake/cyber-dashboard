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
	if _, err := loader.Load(ctx, server.URL, server.URL); err != nil {
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
	if _, err := loader.Load(ctx, server.URL, server.URL); err != nil {
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
