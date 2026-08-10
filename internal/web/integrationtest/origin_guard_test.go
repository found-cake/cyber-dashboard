package integrationtest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIsRejected_whenOriginIsAnotherSite(t *testing.T) {
	// Given a dashboard reachable from the browser of a user who visits a malicious page.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When that page submits a request the browser sends without a preflight.
	request := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(`{"day":"2026-08-09"}`))
	request.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the server refuses it before the handler runs.
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestRequestIsAccepted_whenOriginMatchesTheDashboard(t *testing.T) {
	// Given the bundled frontend loaded from the dashboard itself.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When it calls the API with the origin the browser attaches.
	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	request.Header.Set("Origin", "http://"+request.Host)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the request is served normally.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestRequestIsRejected_whenOriginUsesHTTPSForHTTPDashboard(t *testing.T) {
	// Given the dashboard serves HTTP on an otherwise matching trusted authority.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When a request claims the HTTPS origin for that authority.
	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	request.Header.Set("Origin", "https://"+request.Host)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the different scheme is rejected as a cross-origin request.
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestRequestIsAccepted_whenOriginIsAbsent(t *testing.T) {
	// Given a client such as curl that sends no origin.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When it calls the API.
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the request is served normally.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestRequestIsRejected_whenOriginAndHostMatchUntrustedSite(t *testing.T) {
	// Given a DNS-rebound request whose Origin and Host agree on an untrusted name.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When the malicious page calls the local API through that name.
	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	request.Host = "rebound.example"
	request.Header.Set("Origin", "http://rebound.example")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then matching attacker-controlled headers do not satisfy the trusted-host policy.
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestRequestIsRejected_whenOriginUsesAnotherPortOnTrustedHost(t *testing.T) {
	// Given a trusted hostname served by an unrelated origin on another port.
	server, _, _ := newTestServer(t, &stubFetcher{})

	// When that origin submits a request to the dashboard port.
	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	request.Host = "example.com:8080"
	request.Header.Set("Origin", "http://example.com:8443")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then hostname trust does not replace exact same-origin authority matching.
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}
