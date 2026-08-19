//go:build browser

package browsertest

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
)

type cveNavigationServer struct {
	*httptest.Server
	mu                  sync.Mutex
	cves                []api.CVEInsight
	requests            []string
	revision            uint64
	failNextPage        bool
	failed              bool
	staleOnceOnNextPage bool
}

func newCVENavigationServer(t *testing.T, cves []api.CVEInsight) *cveNavigationServer {
	t.Helper()
	result := &cveNavigationServer{cves: slices.Clone(cves), revision: 1}
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Sources:       []api.Source{},
			Reports:       []api.ReportSummary{},
			Settings:      api.SettingsResponse{Language: "ko"},
			LLMPresets:    []api.LLMPresetResponse{},
			CollectedDays: []string{},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		preview := cves[:min(len(cves), dashboard.DashboardCVELimit)]
		writeJSON(t, writer, api.Dashboard{Total: 12, CVECount: len(cves), CVEs: preview})
	})
	mux.HandleFunc("GET /api/cves", func(writer http.ResponseWriter, request *http.Request) {
		result.mu.Lock()
		result.requests = append(result.requests, request.URL.RequestURI())
		cursor := request.URL.Query().Get("cursor")
		if cursor != "" && result.failNextPage && !result.failed {
			result.failed = true
			result.mu.Unlock()
			writeJSONStatus(t, writer, http.StatusInternalServerError, api.ErrorResponse{Code: "internal", MessageEN: "CVE page failed"})
			return
		}
		if cursor != "" && result.staleOnceOnNextPage {
			result.staleOnceOnNextPage = false
			result.revision++
			result.cves[len(result.cves)-1].CVSS = 10
			result.mu.Unlock()
			writeJSONStatus(t, writer, http.StatusConflict, api.ErrorResponse{Code: "cve_page_stale", MessageEN: "CVE ranking changed"})
			return
		}
		values := slices.Clone(result.cves)
		revision := result.revision
		result.mu.Unlock()
		page := cveFixturePage(values, request)
		writer.Header().Set("X-CVE-Revision", strconv.FormatUint(revision, 10))
		if len(page) > 0 {
			writer.Header().Set("X-CVE-Cursor", request.URL.Query().Get("sort")+"."+page[len(page)-1].ID)
		}
		writeJSON(t, writer, page)
	})
	mux.Handle("/", staticFiles)
	result.Server = httptest.NewServer(mux)
	t.Cleanup(result.Close)
	return result
}

func (s *cveNavigationServer) failNextContinuation() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNextPage = true
}

func (s *cveNavigationServer) staleNextContinuation() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.staleOnceOnNextPage = true
}

func (s *cveNavigationServer) cveRequests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func cveFixturePage(cves []api.CVEInsight, request *http.Request) []api.CVEInsight {
	values := slices.Clone(cves)
	sortKey := request.URL.Query().Get("sort")
	slices.SortFunc(values, func(left, right api.CVEInsight) int {
		byRisk := func() int {
			if score := cmp.Compare(right.CVSS+float64(right.Mentions)*0.2, left.CVSS+float64(left.Mentions)*0.2); score != 0 {
				return score
			}
			if firstSeen := cmp.Compare(right.FirstSeen, left.FirstSeen); firstSeen != 0 {
				return firstSeen
			}
			return cmp.Compare(left.ID, right.ID)
		}
		switch sortKey {
		case "cvss":
			return cmp.Or(cmp.Compare(right.CVSS, left.CVSS), byRisk())
		case "mentions":
			return cmp.Or(cmp.Compare(right.Mentions, left.Mentions), byRisk())
		case "firstSeen":
			return cmp.Or(cmp.Compare(right.FirstSeen, left.FirstSeen), byRisk())
		default:
			return byRisk()
		}
	})
	start := 0
	if cursor := request.URL.Query().Get("cursor"); cursor != "" {
		_, id, found := strings.Cut(cursor, ".")
		if !found {
			return []api.CVEInsight{}
		}
		index := slices.IndexFunc(values, func(value api.CVEInsight) bool { return value.ID == id })
		if index < 0 {
			return []api.CVEInsight{}
		}
		start = index + 1
	}
	return values[start:min(start+dashboard.CVEPageSize, len(values))]
}
