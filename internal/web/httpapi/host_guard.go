package httpapi

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
)

type hostGuard struct {
	trusted            map[string]struct{}
	allowUntrustedHost bool
}

func newHostGuard(hosts []string, allowUntrustedHost bool) *hostGuard {
	trusted := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		trusted[strings.ToLower(host)] = struct{}{}
	}
	return &hostGuard{trusted: trusted, allowUntrustedHost: allowUntrustedHost}
}

func (g *hostGuard) middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		requestHost, err := normalizeTrustedHost(c.Request().Host)
		if err != nil || requestHost == "" || (!g.allowUntrustedHost && !g.allows(requestHost)) {
			return rejectUntrustedHost(c)
		}
		origin := c.Request().Header.Get("Origin")
		if origin == "" {
			return next(c)
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "http" || parsed.User != nil ||
			!strings.EqualFold(parsed.Host, c.Request().Host) {
			return rejectUntrustedHost(c)
		}
		return next(c)
	}
}

func (g *hostGuard) allows(host string) bool {
	if isLoopbackHost(host) {
		return true
	}
	_, ok := g.trusted[host]
	return ok
}

func NormalizeTrustedHost(value string) (string, error) {
	return normalizeTrustedHost(value)
}

func normalizeTrustedHost(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", nil
	}
	// net.SplitHostPort strips the brackets from an IPv6 address, which url.Parse then rejects.
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String(), nil
	}
	if strings.Contains(raw, "://") {
		return "", fmt.Errorf("trusted host must be a hostname or IP address")
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("trusted host must be a hostname or IP address")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || !validHostname(host) {
		return "", fmt.Errorf("trusted host must be a hostname or IP address")
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	return host, nil
}

func validHostname(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func rejectUntrustedHost(c *echo.Context) error {
	return c.JSON(http.StatusForbidden, localizedError("cross_origin_request",
		"신뢰하지 않는 호스트 또는 다른 사이트의 요청은 처리하지 않습니다",
		"Requests from untrusted hosts or other sites are not accepted"))
}
