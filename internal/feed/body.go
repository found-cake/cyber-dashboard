package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const maximumArticlePageBytes = 8 << 20

var ErrArticleFiltered = errors.New("article filtered by source policy")

type BrowserBodyLoader interface {
	Load(ctx context.Context, articleURL string) (string, error)
}

type ArticleBodyLoader struct {
	client  *http.Client
	browser BrowserBodyLoader
}

func NewArticleBodyLoader(client *http.Client, browser BrowserBodyLoader) *ArticleBodyLoader {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &ArticleBodyLoader{client: client, browser: browser}
}

func (l *ArticleBodyLoader) Load(ctx context.Context, source api.Source, article FeedArticle) (string, error) {
	if source.Slug == "cybersecuritynews" {
		return embeddedArticleBody(article)
	}
	switch source.Slug {
	case "darkreading", "bleepingcomputer":
		if l.browser == nil {
			return "", fmt.Errorf("Chromium is unavailable for %s", source.Slug)
		}
		return l.browser.Load(ctx, article.URL)
	}
	return l.loadHTTP(ctx, source.Slug, article.URL)
}

func (l *ArticleBodyLoader) loadHTTP(ctx context.Context, sourceSlug, articleURL string) (string, error) {
	parsed, err := url.ParseRequestURI(articleURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("invalid article URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create article request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/138 Safari/537.36")
	response, err := l.client.Do(request)
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

func embeddedArticleBody(article FeedArticle) (string, error) {
	raw, ok := article.SourceMetadata["cybersecuritynews"]
	if !ok {
		return "", fmt.Errorf("Cybersecurity News content is missing")
	}
	var metadata struct {
		Content string `json:"content_encoded"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "", fmt.Errorf("decode Cybersecurity News content: %w", err)
	}
	return extractArticleText(metadata.Content, "")
}

func extractArticleText(markup, sourceSlug string) (string, error) {
	document, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return "", err
	}
	if sourceSlug == "stepsecurity" && strings.EqualFold(stepSecurityCategory(document), "Product") {
		return "", ErrArticleFiltered
	}
	root := contentRoot(document, sourceSlug)
	var builder strings.Builder
	writeFlowText(root, &builder)
	paragraphs := normalizeParagraphs(builder.String())
	if len(paragraphs) == 0 {
		return "", fmt.Errorf("article content container is empty")
	}
	return strings.Join(paragraphs, "\n\n"), nil
}

func stepSecurityCategory(document *html.Node) string {
	article := findElement(document, "article")
	if article == nil {
		return ""
	}
	header := findElement(article, ".blog-post-header_left-column")
	if header == nil {
		return ""
	}
	margin := findElement(header, ".margin-bottom")
	if margin == nil {
		return ""
	}
	link := findElement(margin, "a")
	if link == nil {
		return ""
	}
	badge := findElement(link, "div")
	if badge == nil {
		return ""
	}
	var value strings.Builder
	writeFlowText(badge, &value)
	return strings.Join(strings.Fields(value.String()), " ")
}

func contentRoot(document *html.Node, sourceSlug string) *html.Node {
	selectors := map[string][]string{
		"boannews":         {"#news_content"},
		"thehackernews":    {"#articlebody", ".articlebody"},
		"stepsecurity":     {".blog-post-content_description"},
		"bleepingcomputer": {".articleBody", ".article-body"},
	}
	for _, selector := range selectors[sourceSlug] {
		if node := findElement(document, selector); node != nil {
			return node
		}
	}
	for _, selector := range []string{"article", "main", "body"} {
		if node := findElement(document, selector); node != nil {
			return node
		}
	}
	return document
}

func findElement(node *html.Node, selector string) *html.Node {
	if matchesElement(node, selector) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if match := findElement(child, selector); match != nil {
			return match
		}
	}
	return nil
}

func matchesElement(node *html.Node, selector string) bool {
	if node.Type != html.ElementNode {
		return false
	}
	if !strings.HasPrefix(selector, ".") && !strings.HasPrefix(selector, "#") {
		return strings.EqualFold(node.Data, selector)
	}
	attributeName := "class"
	value := strings.TrimPrefix(selector, ".")
	if strings.HasPrefix(selector, "#") {
		attributeName = "id"
		value = strings.TrimPrefix(selector, "#")
	}
	for _, attribute := range node.Attr {
		if attribute.Key == attributeName && (attributeName == "id" && attribute.Val == value || attributeName == "class" && containsWord(attribute.Val, value)) {
			return true
		}
	}
	return false
}

func writeFlowText(node *html.Node, builder *strings.Builder) {
	if node.Type == html.ElementNode && isIgnoredElement(node.Data) {
		return
	}
	if node.Type == html.TextNode {
		builder.WriteString(" ")
		builder.WriteString(node.Data)
		return
	}
	if node.Type == html.ElementNode && node.Data == "br" {
		builder.WriteString("\n\n")
		return
	}
	isBlock := node.Type == html.ElementNode && isTextBlock(node.Data)
	if isBlock {
		builder.WriteString("\n\n")
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeFlowText(child, builder)
	}
	if isBlock {
		builder.WriteString("\n\n")
	}
}

func normalizeParagraphs(value string) []string {
	lines := strings.Split(strings.ReplaceAll(value, "\r", "\n"), "\n")
	paragraphs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			paragraphs = append(paragraphs, line)
		}
	}
	return paragraphs
}

func isTextBlock(tag string) bool {
	switch strings.ToLower(tag) {
	case "p", "h2", "h3", "h4", "li", "blockquote", "pre":
		return true
	default:
		return false
	}
}

func isIgnoredElement(tag string) bool {
	switch strings.ToLower(tag) {
	case "script", "style", "noscript", "svg", "nav", "footer", "form":
		return true
	default:
		return false
	}
}

func containsWord(value, word string) bool {
	for _, candidate := range strings.Fields(value) {
		if candidate == word {
			return true
		}
	}
	return false
}
