package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
)

func TestRefreshCVEsReturnsRefreshResult(t *testing.T) {
	// Given the NVD refresh service has updated and removed stored CVEs.
	enricher := &stubVulnerabilityEnricher{result: api.CVERefreshResult{Updated: 4, Removed: 1, Warnings: []string{}}}
	server, _, _ := newTestServerWithVulnerability(t, &stubFetcher{}, "test-nvd-key", enricher)

	// When the manual refresh endpoint is requested.
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/cves/refresh", nil))

	// Then the public API returns the refresh counts.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var result api.CVERefreshResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if enricher.refreshCalls != 1 || result.Updated != 4 || result.Removed != 1 {
		t.Fatalf("calls = %d, result = %+v", enricher.refreshCalls, result)
	}
}

func TestRefreshCVEsReturnsBilingualPrecondition_whenNVDKeyIsMissing(t *testing.T) {
	// Given refresh cannot run because no NVD API key is configured.
	enricher := &stubVulnerabilityEnricher{err: vulnerability.ErrAPIKeyRequired}
	server, _, _ := newTestServerWithVulnerability(t, &stubFetcher{}, "", enricher)

	// When the manual refresh endpoint is requested.
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/cves/refresh", nil))

	// Then both Korean and English guidance are returned to the user.
	if recorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response api.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if response.Code != "nvd_key_required" || response.MessageKO == "" || response.MessageEN == "" {
		t.Fatalf("response = %+v", response)
	}
}

type stubVulnerabilityEnricher struct {
	result       api.CVERefreshResult
	err          error
	refreshCalls int
}

func (*stubVulnerabilityEnricher) EnrichDay(context.Context, string) error {
	return nil
}

func (s *stubVulnerabilityEnricher) RefreshAll(context.Context) (api.CVERefreshResult, error) {
	s.refreshCalls++
	return s.result, s.err
}
