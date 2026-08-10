package store

import (
	"html"
	"regexp"
	"strings"
)

var (
	cvePattern = regexp.MustCompile(`(?i)\bCVE-\d{4}-\d{4,}\b`)
	tagPattern = regexp.MustCompile(`<[^>]*>`)
)

func cleanText(value string) string {
	withoutTags := tagPattern.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(html.UnescapeString(withoutTags)), " ")
}

func extractCVEs(value string) []string {
	matches := cvePattern.FindAllString(value, -1)
	seen := make(map[string]struct{}, len(matches))
	cves := make([]string, 0, len(matches))
	for _, match := range matches {
		cve := strings.ToUpper(match)
		if _, exists := seen[cve]; exists {
			continue
		}
		seen[cve] = struct{}{}
		cves = append(cves, cve)
	}
	return cves
}
