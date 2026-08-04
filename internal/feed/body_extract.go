package feed

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

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
