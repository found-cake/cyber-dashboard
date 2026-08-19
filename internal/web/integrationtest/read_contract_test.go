package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
)

func TestReadEndpointsExposeSlimBootstrapAndFullDetails(t *testing.T) {
	// Given persisted report data and more CVEs than the dashboard preview allows.
	upstream := compatibleLLM(t, "Complete report summary")
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	configureLLM(t, appSettings, upstream.URL)
	day := recentCollectionDay()
	for index := range 10 {
		cveID := fmt.Sprintf("CVE-2026-%04d", index)
		if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
			ID: "sha256:read-contract-" + cveID, URL: "https://example.com/" + cveID,
			Title: cveID + " advisory", Description: "Assessment for " + cveID,
		}, day); err != nil {
			t.Fatalf("save article %s: %v", cveID, err)
		}
		if err := feeds.SaveAssessment(context.Background(), vulnerability.Assessment{
			CVEID: cveID, Score: 9.9 - float64(index)*0.5, Product: "Product " + cveID,
		}); err != nil {
			t.Fatalf("save assessment %s: %v", cveID, err)
		}
	}
	if err := feeds.SaveDailySummary(context.Background(), day, "Daily report evidence"); err != nil {
		t.Fatalf("save daily summary: %v", err)
	}
	createdResponse := performRequest(t, server, http.MethodPost, "/api/reports", api.CreateReportRequest{
		Type: "weekly", Start: day, End: day,
	})
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create report status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created api.Report
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created report: %v", err)
	}

	// When the optimized read endpoints are requested through the real router.
	cvesResponse := performRequest(t, server, http.MethodGet, "/api/cves", nil)
	bootstrapResponse := performRequest(t, server, http.MethodGet, "/api/bootstrap", nil)
	detailResponse := performRequest(t, server, http.MethodGet, fmt.Sprintf("/api/reports/%d", created.ID), nil)

	// Then the explorer is complete, bootstrap is metadata-only, and detail round-trips the body.
	if cvesResponse.Code != http.StatusOK {
		t.Fatalf("CVE status = %d, body = %s", cvesResponse.Code, cvesResponse.Body.String())
	}
	var cves []api.CVEInsight
	if err := json.Unmarshal(cvesResponse.Body.Bytes(), &cves); err != nil {
		t.Fatalf("decode CVEs: %v", err)
	}
	if len(cves) != 10 || len(cves) <= dashboard.DashboardCVELimit {
		t.Fatalf("CVE count = %d, want complete list beyond preview limit", len(cves))
	}
	if cves[0].ID != "CVE-2026-0000" || cves[0].CVSS != 9.9 || cves[0].AffectedProduct != "Product CVE-2026-0000" || cves[0].Mentions != 1 {
		t.Fatalf("leading CVE = %+v", cves[0])
	}

	var bootstrap struct {
		Reports []map[string]json.RawMessage `json:"reports"`
	}
	if bootstrapResponse.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d, body = %s", bootstrapResponse.Code, bootstrapResponse.Body.String())
	}
	if err := json.Unmarshal(bootstrapResponse.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	wantSummaryKeys := map[string]bool{"id": true, "type": true, "period_start": true, "period_end": true}
	if len(bootstrap.Reports) != 1 || len(bootstrap.Reports[0]) != len(wantSummaryKeys) {
		t.Fatalf("bootstrap reports = %+v", bootstrap.Reports)
	}
	for key := range bootstrap.Reports[0] {
		if !wantSummaryKeys[key] {
			t.Fatalf("bootstrap report contains body field %q", key)
		}
	}

	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail api.Report
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode report detail: %v", err)
	}
	if !reflect.DeepEqual(detail, created) || detail.Summary == "" || detail.Total != 10 {
		t.Fatalf("detail = %+v, created = %+v", detail, created)
	}
}

func TestReportDetailRejectsInvalidAndMissingIDs(t *testing.T) {
	// Given malformed, out-of-range, and absent report identifiers.
	server, _, _ := newTestServer(t, &stubFetcher{})
	tests := []struct {
		name string
		path string
		code int
		kind string
	}{
		{name: "text", path: "/api/reports/not-a-number", code: http.StatusBadRequest, kind: "bad_request"},
		{name: "zero", path: "/api/reports/0", code: http.StatusBadRequest, kind: "bad_request"},
		{name: "negative", path: "/api/reports/-1", code: http.StatusBadRequest, kind: "bad_request"},
		{name: "overflow", path: "/api/reports/9223372036854775808", code: http.StatusBadRequest, kind: "bad_request"},
		{name: "missing", path: "/api/reports/999999", code: http.StatusNotFound, kind: "not_found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When the identifier reaches the public detail route.
			response := performRequest(t, server, http.MethodGet, test.path, nil)

			// Then it returns the stable client-facing error without leaking report data.
			if response.Code != test.code {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.code, response.Body.String())
			}
			var value api.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if value.Code != test.kind || value.MessageKO == "" || value.MessageEN == "" {
				t.Fatalf("error response = %+v", value)
			}
		})
	}
}

func TestCVEEndpointCapsPages_withoutHidingLaterEntries(t *testing.T) {
	// Given one more CVE than a single public response may contain.
	server, feeds, _ := newTestServer(t, &stubFetcher{})
	day := recentCollectionDay()
	for index := range dashboard.CVEPageSize + 1 {
		cveID := fmt.Sprintf("CVE-2026-%04d", index)
		if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
			ID: "sha256:cve-page-" + cveID, URL: "https://example.com/" + cveID,
			Title: cveID + " advisory", Description: "Assessment for " + cveID,
		}, day); err != nil {
			t.Fatalf("save article %s: %v", cveID, err)
		}
	}

	// When consecutive cursors are requested through the real router.
	firstResponse := performRequest(t, server, http.MethodGet, "/api/cves?sort=score", nil)
	revision := firstResponse.Header().Get("X-CVE-Revision")
	cursor := firstResponse.Header().Get("X-CVE-Cursor")
	secondResponse := performRequest(t, server, http.MethodGet, fmt.Sprintf(
		"/api/cves?sort=score&cursor=%s&revision=%s", url.QueryEscape(cursor), url.QueryEscape(revision)), nil)

	// Then the first response is capped and the remaining entry is available on the next page.
	if firstResponse.Code != http.StatusOK || secondResponse.Code != http.StatusOK {
		t.Fatalf("statuses = %d and %d; bodies = %s and %s", firstResponse.Code, secondResponse.Code, firstResponse.Body.String(), secondResponse.Body.String())
	}
	var first, second []api.CVEInsight
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first CVE page: %v", err)
	}
	if err := json.Unmarshal(secondResponse.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second CVE page: %v", err)
	}
	if revision == "" || cursor == "" || len(first) != dashboard.CVEPageSize || len(second) != 1 {
		t.Fatalf("page lengths = %d and %d", len(first), len(second))
	}
}

func TestCVEEndpointRejectsInvalidPageQueries(t *testing.T) {
	// Given query values outside the public CVE page contract.
	server, _, _ := newTestServer(t, &stubFetcher{})
	tests := []string{
		"/api/cves?sort=unknown",
		"/api/cves?offset=-1",
		"/api/cves?cursor=score.CVE-2026-0001",
		"/api/cves?cursor=score.CVE-2026-0001&revision=text",
		"/api/cves?cursor=other.CVE-2026-0001&revision=1",
		"/api/cves?cursor=score.CVE-2026-9999&revision=1",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// When the malformed query reaches the real router.
			response := performRequest(t, server, http.MethodGet, path, nil)

			// Then the server returns the stable client-facing bad-request shape.
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var value api.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if value.Code != "bad_request" || value.MessageKO == "" || value.MessageEN == "" {
				t.Fatalf("error response = %+v", value)
			}
		})
	}
}

func TestCVEEndpointRejectsStaleRevisionAndInvalidCursor(t *testing.T) {
	// Given a first page whose server ranking changes before its continuation.
	server, feeds, _ := newTestServer(t, &stubFetcher{})
	day := recentCollectionDay()
	for index := range dashboard.CVEPageSize + 1 {
		cveID := fmt.Sprintf("CVE-2026-%04d", index)
		if err := feeds.SaveArticle(context.Background(), api.Source{ID: 1}, collector.FeedArticle{
			ID: "sha256:cve-revision-" + cveID, URL: "https://example.com/" + cveID,
			Title: cveID + " advisory", Description: "Assessment for " + cveID,
		}, day); err != nil {
			t.Fatalf("save article %s: %v", cveID, err)
		}
	}
	first := performRequest(t, server, http.MethodGet, "/api/cves?sort=score", nil)
	revision := first.Header().Get("X-CVE-Revision")
	cursor := first.Header().Get("X-CVE-Cursor")
	if revision == "" || cursor == "" {
		t.Fatal("first CVE page omitted its continuation headers")
	}
	if err := feeds.SaveAssessment(context.Background(), vulnerability.Assessment{
		CVEID: "CVE-2026-0100", Score: 10, Product: "Promoted product",
	}); err != nil {
		t.Fatalf("change CVE ranking: %v", err)
	}

	// When the stale next page and an unknown cursor are requested.
	stale := performRequest(t, server, http.MethodGet, fmt.Sprintf(
		"/api/cves?sort=score&cursor=%s&revision=%s", url.QueryEscape(cursor), url.QueryEscape(revision)), nil)
	restarted := performRequest(t, server, http.MethodGet, "/api/cves?sort=score", nil)
	currentRevision := restarted.Header().Get("X-CVE-Revision")
	invalidCursor := performRequest(t, server, http.MethodGet, fmt.Sprintf(
		"/api/cves?sort=score&cursor=score.CVE-2026-9999&revision=%s", url.QueryEscape(currentRevision)), nil)

	// Then stale data is conflict-rejected and unknown cursor state is not accepted.
	if stale.Code != http.StatusConflict || invalidCursor.Code != http.StatusBadRequest {
		t.Fatalf("statuses = stale %d, invalid cursor %d", stale.Code, invalidCursor.Code)
	}
	var value api.ErrorResponse
	if err := json.Unmarshal(stale.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode stale response: %v", err)
	}
	if value.Code != "cve_page_stale" {
		t.Fatalf("stale response = %+v", value)
	}
}
