package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
