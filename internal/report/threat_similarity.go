package report

import (
	"net/url"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func sameThreat(left, right threatArticle) bool {
	leftURL, rightURL := canonicalThreatURL(left.URL), canonicalThreatURL(right.URL)
	if leftURL != "" && leftURL == rightURL {
		return true
	}
	if cvesOverlap(splitCVEs(left.CVEs), splitCVEs(right.CVEs)) {
		return true
	}
	return titlesMatch(left.Title, right.Title)
}

func canonicalThreatURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || slices.Contains([]string{"fbclid", "gclid", "mc_cid", "mc_eid"}, lower) {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	return strings.TrimSuffix(parsed.String(), "/")
}

func splitCVEs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	values := strings.Split(value, ",")
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
	}
	slices.Sort(values)
	return slices.Compact(values)
}

func cvesOverlap(left, right []string) bool {
	for _, leftCVE := range left {
		if slices.Contains(right, leftCVE) {
			return true
		}
	}
	return false
}

func titlesMatch(left, right string) bool {
	leftNormalized, rightNormalized := normalizeThreatTitle(left), normalizeThreatTitle(right)
	if leftNormalized == "" || rightNormalized == "" {
		return false
	}
	if leftNormalized == rightNormalized {
		return true
	}
	leftTokens, rightTokens := strings.Fields(leftNormalized), strings.Fields(rightNormalized)
	if numericTokensConflict(leftTokens, rightTokens) {
		return false
	}
	if len(leftTokens) >= 4 && len(rightTokens) >= 4 && tokenSimilarity(leftTokens, rightTokens) >= 0.85 {
		return true
	}
	return trigramSimilarity(leftNormalized, rightNormalized) >= 0.92
}

func numericTokensConflict(left, right []string) bool {
	leftNumbers, rightNumbers := numericTokens(left), numericTokens(right)
	return (len(leftNumbers) > 0 || len(rightNumbers) > 0) && !slices.Equal(leftNumbers, rightNumbers)
}

func numericTokens(tokens []string) []string {
	values := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if strings.ContainsFunc(token, unicode.IsNumber) {
			values = append(values, token)
		}
	}
	slices.Sort(values)
	return values
}

func normalizeThreatTitle(value string) string {
	value = strings.ToLower(norm.NFKC.String(value))
	return strings.Join(strings.FieldsFunc(value, func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsNumber(character)
	}), " ")
}

func tokenSimilarity(left, right []string) float64 {
	leftSet, rightSet := make(map[string]struct{}, len(left)), make(map[string]struct{}, len(right))
	for _, token := range left {
		leftSet[token] = struct{}{}
	}
	for _, token := range right {
		rightSet[token] = struct{}{}
	}
	common := 0
	for token := range leftSet {
		if _, ok := rightSet[token]; ok {
			common++
		}
	}
	return float64(common) / float64(len(leftSet)+len(rightSet)-common)
}

func trigramSimilarity(left, right string) float64 {
	leftRunes := []rune(strings.ReplaceAll(left, " ", ""))
	rightRunes := []rune(strings.ReplaceAll(right, " ", ""))
	if len(leftRunes) < 20 || len(rightRunes) < 20 {
		return 0
	}
	leftSet, rightSet := trigrams(leftRunes), trigrams(rightRunes)
	common := 0
	for value := range leftSet {
		if _, ok := rightSet[value]; ok {
			common++
		}
	}
	return float64(2*common) / float64(len(leftSet)+len(rightSet))
}

func trigrams(value []rune) map[string]struct{} {
	result := make(map[string]struct{}, len(value)-2)
	for index := 0; index <= len(value)-3; index++ {
		result[string(value[index:index+3])] = struct{}{}
	}
	return result
}
