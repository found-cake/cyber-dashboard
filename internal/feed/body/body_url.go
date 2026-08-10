package body

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

type articleURLPolicy struct {
	hostname string
	port     string
}

type validatedArticleURL struct {
	url    *url.URL
	policy articleURLPolicy
}

func newArticleURLPolicy(sourceHost string) (articleURLPolicy, error) {
	policy, err := parseArticleURLPolicy(sourceHost)
	if err != nil {
		return articleURLPolicy{}, err
	}
	if isPrivateNetworkHost(policy.hostname) {
		return articleURLPolicy{}, fmt.Errorf("source host points to a private network")
	}
	return policy, nil
}

func parseArticleURLPolicy(sourceHost string) (articleURLPolicy, error) {
	sourceHost = strings.TrimSpace(sourceHost)
	if sourceHost == "" {
		return articleURLPolicy{}, fmt.Errorf("source host is missing")
	}
	if !strings.Contains(sourceHost, "://") {
		sourceHost = "https://" + sourceHost
	}
	parsed, err := url.Parse(sourceHost)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return articleURLPolicy{}, fmt.Errorf("source host is invalid")
	}
	return articleURLPolicy{
		hostname: canonicalHostname(parsed.Hostname()),
		port:     parsed.Port(),
	}, nil
}

func (p articleURLPolicy) validate(target *url.URL, rejectPrivateNetwork bool) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		return fmt.Errorf("URL must use HTTP or HTTPS")
	}
	if target.User != nil {
		return fmt.Errorf("URL credentials are not allowed")
	}
	if canonicalHostname(target.Hostname()) != p.hostname {
		return fmt.Errorf("URL host does not match the configured source")
	}
	if p.port != "" {
		if target.Port() != p.port {
			return fmt.Errorf("URL port does not match the configured source")
		}
	} else if port := target.Port(); port != "" && port != defaultPort(target.Scheme) {
		return fmt.Errorf("URL uses a non-default port")
	}
	if rejectPrivateNetwork && isPrivateNetworkHost(target.Hostname()) {
		return fmt.Errorf("URL points to a private network")
	}
	return nil
}

func (p articleURLPolicy) authority() string {
	if p.port == "" {
		return p.hostname
	}
	return p.hostname + ":" + p.port
}

func canonicalHostname(hostname string) string {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	return strings.TrimPrefix(hostname, "www.")
}

func defaultPort(scheme string) string {
	if scheme == "http" {
		return "80"
	}
	return "443"
}

func isPrivateNetworkHost(hostname string) bool {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || strings.HasSuffix(hostname, ".local") {
		return true
	}
	address, err := netip.ParseAddr(hostname)
	if err != nil {
		return false
	}
	address = address.Unmap()
	if address.IsPrivate() || address.IsLoopback() || address.IsUnspecified() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
		return true
	}
	return netip.MustParsePrefix("100.64.0.0/10").Contains(address)
}
