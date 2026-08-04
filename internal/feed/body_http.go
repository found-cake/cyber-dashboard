package feed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html/charset"
)

func (l *ArticleBodyLoader) loadHTTP(ctx context.Context, sourceSlug string, target validatedArticleURL) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.url.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create article request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/138 Safari/537.36")
	client := *l.client
	configuredRedirectPolicy := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := target.policy.validate(request.URL, true); err != nil {
			return fmt.Errorf("reject article redirect: %w", err)
		}
		if configuredRedirectPolicy != nil {
			return configuredRedirectPolicy(request, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch article: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumArticlePageBytes+1))
	if err != nil {
		return "", fmt.Errorf("read article response: %w", err)
	}
	if len(body) > maximumArticlePageBytes {
		return "", fmt.Errorf("article response exceeds %d bytes", maximumArticlePageBytes)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch article: status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	decoded, err := charset.NewReader(bytes.NewReader(body), response.Header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("decode article charset: %w", err)
	}
	decodedBody, err := io.ReadAll(decoded)
	if err != nil {
		return "", fmt.Errorf("read decoded article: %w", err)
	}
	value, err := extractArticleText(string(decodedBody), sourceSlug)
	if err != nil {
		return "", fmt.Errorf("extract article body: %w", err)
	}
	return value, nil
}
