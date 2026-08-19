//go:build browser

package browsertest

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
)

type cveNavigationServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

func newCVENavigationServer(t *testing.T, cves []api.CVEInsight) *cveNavigationServer {
	t.Helper()
	result := &cveNavigationServer{}
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
		result.mu.Unlock()
		writeJSON(t, writer, cveFixturePage(cves, request))
	})
	mux.Handle("/", staticFiles)
	result.Server = httptest.NewServer(mux)
	t.Cleanup(result.Close)
	return result
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
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	if offset >= len(values) {
		return []api.CVEInsight{}
	}
	return values[offset:min(offset+dashboard.CVEPageSize, len(values))]
}
