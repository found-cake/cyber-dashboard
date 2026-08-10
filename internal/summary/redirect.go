package summary

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

var errCrossOriginRedirect = errors.New("LLM redirect changed origin")

func checkSameOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 || sameOrigin(via[0].URL, request.URL) {
		return nil
	}
	return errCrossOriginRedirect
}

func sameOrigin(first, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) &&
		strings.EqualFold(first.Hostname(), second.Hostname()) &&
		originPort(first) == originPort(second)
}

func originPort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
