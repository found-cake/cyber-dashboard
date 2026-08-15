//go:build browser

package browsertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

// Tagging the nodes lets the assertions tell a surviving element from an identical rebuild.
const markPartialRenderProbes = `(() => {
	document.querySelectorAll("[data-probe]").forEach(node => node.removeAttribute("data-probe"));
	document.querySelector(".cve-overview-card table").dataset.probe = "cve";
	document.querySelector("#dashboard-stats").dataset.probe = "stats";
	document.querySelector("#attack-method-bars").dataset.probe = "methods";
})()`

const readPartialRenderProbes = `({
	cveTableKept: document.querySelector(".cve-overview-card table").dataset.probe === "cve",
	statsKept: document.querySelector("#dashboard-stats").dataset.probe === "stats",
	methodBarsKept: document.querySelector("#attack-method-bars").dataset.probe === "methods",
	focusKept: document.activeElement === document.querySelector("#hide-none-actor")
		|| document.activeElement === document.querySelector("#dashboard-range"),
	firstActor: document.querySelector("#threat-actor-bars .bar-label").textContent
})`

func newDashboardPartialRenderServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := newDashboardBrowserServer(t, func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, dashboardBrowserFixture(request, 13))
	})
	return server, &requests
}

func newDashboardInitialRaceServer(t *testing.T) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	var requests atomic.Int32
	initialStarted := make(chan struct{})
	releaseInitial := make(chan struct{})
	var release sync.Once
	server := newDashboardBrowserServer(t, func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 1 {
			close(initialStarted)
			<-releaseInitial
		}
		total := 30
		if request.URL.Query().Get("days") == "7" {
			total = 7
		}
		writeJSON(t, writer, dashboardBrowserFixture(request, total))
	})
	releaseRequest := func() { release.Do(func() { close(releaseInitial) }) }
	t.Cleanup(releaseRequest)
	return server, initialStarted, releaseRequest
}

func newDashboardOverlapServer(t *testing.T) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	var requests atomic.Int32
	rangeStarted := make(chan struct{})
	releaseRange := make(chan struct{})
	var release sync.Once
	server := newDashboardBrowserServer(t, func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 2 {
			close(rangeStarted)
			<-releaseRange
			http.Error(writer, "superseded range failed", http.StatusInternalServerError)
			return
		}
		writeJSON(t, writer, dashboardBrowserFixture(request, 13))
	})
	releaseRequest := func() { release.Do(func() { close(releaseRange) }) }
	t.Cleanup(releaseRequest)
	return server, rangeStarted, releaseRequest
}

func newDashboardBrowserServer(t *testing.T, dashboard http.HandlerFunc) *httptest.Server {
	return newDashboardBrowserServerForLanguage(t, dashboard, "en")
}

func newDashboardBrowserServerForLanguage(t *testing.T, dashboard http.HandlerFunc, language string) *httptest.Server {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Settings: api.SettingsResponse{Language: language}})
	})
	mux.HandleFunc("GET /api/dashboard", dashboard)
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func dashboardBrowserFixture(request *http.Request, total int) api.Dashboard {
	actors := []api.BreakdownRow{{Label: "None", Value: 9}, {Label: "LockBit", Value: 4}}
	if request.URL.Query().Get("hide_none") == "1" {
		actors = []api.BreakdownRow{{Label: "LockBit", Value: 4}}
	}
	return api.Dashboard{
		Total: total, Critical: 2, High: 3, CVECount: 1,
		AttackMethods: []api.BreakdownRow{{Label: "Ransomware", Value: 4}},
		ThreatActors:  actors,
		Trend:         dashboardBrowserTrend(request),
		CVEs: []api.CVEInsight{{
			ID: "CVE-2026-1001", CVSS: 8.1, AffectedProduct: "acme / gateway", FirstSeen: "2026-08-01", Mentions: 2,
		}},
	}
}

// dashboardBrowserTrend mirrors the server's bucketing: a longer range widens its buckets.
func dashboardBrowserTrend(request *http.Request) []api.TrendPoint {
	buckets, size := 10, 3
	switch request.URL.Query().Get("days") {
	case "7":
		buckets, size = 7, 1
	case "90":
		buckets, size = 10, 9
	}
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	points := make([]api.TrendPoint, buckets)
	for index := range points {
		bucketStart := start.AddDate(0, 0, index*size)
		points[index] = api.TrendPoint{
			Start: bucketStart.Format(time.DateOnly), End: bucketStart.AddDate(0, 0, size-1).Format(time.DateOnly),
			Total: 6 + index, Critical: 1, High: 2, Medium: 3,
			Attributed: 4, UnknownActor: 2, QualifiedUnknown: 1, NamedActor: 1,
		}
	}
	return points
}

func waitForDashboardRequest(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
