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

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestDashboardControlsRepaintOnlyTheirOwnBlocks(t *testing.T) {
	// Given a dashboard whose threat actors change with the None filter.
	server, requests := newDashboardPartialRenderServer(t)
	browser := newBrowserContext(t, 20*time.Second)

	// When the None filter is toggled after marking the blocks it must not touch.
	var toggled struct {
		CVETableKept   bool   `json:"cveTableKept"`
		StatsKept      bool   `json:"statsKept"`
		MethodBarsKept bool   `json:"methodBarsKept"`
		FocusKept      bool   `json:"focusKept"`
		FirstActor     string `json:"firstActor"`
	}
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#threat-actor-bars`),
		chromedp.Evaluate(markPartialRenderProbes, nil),
		chromedp.Evaluate(`(() => { const toggle = document.querySelector("#hide-none-actor"); toggle.focus(); toggle.click(); })()`, nil),
		chromedp.Poll(`document.querySelector("#threat-actor-bars .bar-label")?.textContent === "LockBit"`, nil),
		chromedp.Evaluate(readPartialRenderProbes, &toggled),
	); err != nil {
		t.Fatalf("toggle the None filter: %v", err)
	}

	// Then only that card's bars were rebuilt, and the switch kept focus.
	if !toggled.CVETableKept || !toggled.StatsKept || !toggled.MethodBarsKept {
		t.Fatalf("None filter re-rendered untouched blocks: %+v", toggled)
	}
	if !toggled.FocusKept || toggled.FirstActor != "LockBit" {
		t.Fatalf("None filter result = %+v", toggled)
	}

	// When the aggregation range changes.
	var ranged struct {
		CVETableKept   bool   `json:"cveTableKept"`
		StatsKept      bool   `json:"statsKept"`
		MethodBarsKept bool   `json:"methodBarsKept"`
		FocusKept      bool   `json:"focusKept"`
		FirstActor     string `json:"firstActor"`
	}
	if err := chromedp.Run(browser,
		chromedp.Evaluate(markPartialRenderProbes, nil),
		chromedp.Evaluate(`(() => {
			const select = document.querySelector("#dashboard-range");
			select.focus();
			select.value = "7";
			select.dispatchEvent(new Event("change", { bubbles: true }));
		})()`, nil),
		chromedp.Poll(`document.querySelector("#threat-actors-card .card-subtitle")?.textContent.includes("7") === true`, nil),
		chromedp.Evaluate(readPartialRenderProbes, &ranged),
	); err != nil {
		t.Fatalf("change the aggregation range: %v", err)
	}

	// Then the CVE card survives, because its data does not depend on the range.
	if !ranged.CVETableKept {
		t.Fatalf("range change re-rendered the CVE card: %+v", ranged)
	}
	if ranged.StatsKept || ranged.MethodBarsKept {
		t.Fatalf("range change left range-dependent blocks stale: %+v", ranged)
	}
	if !ranged.FocusKept {
		t.Fatalf("range change moved focus off the select: %+v", ranged)
	}
	if requests.Load() != 3 {
		t.Fatalf("dashboard requests = %d, want 3", requests.Load())
	}
}

func TestDashboardRangeChangeDuringInitialLoadKeepsTheNewestResponse(t *testing.T) {
	server, initialStarted, releaseInitial := newDashboardInitialRaceServer(t)
	browser := newBrowserContext(t, 20*time.Second)

	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#dashboard-range`),
	); err != nil {
		t.Fatalf("start dashboard load: %v", err)
	}
	waitForDashboardRequest(t, initialStarted, "initial dashboard request")

	if err := chromedp.Run(browser,
		chromedp.Evaluate(`(() => {
			const select = document.querySelector("#dashboard-range");
			select.value = "7";
			select.dispatchEvent(new Event("change", { bubbles: true }));
		})()`, nil),
		chromedp.Poll(`document.querySelector("#dashboard-stats strong")?.textContent === "7"`, nil),
	); err != nil {
		t.Fatalf("render the selected range: %v", err)
	}

	releaseInitial()
	if err := chromedp.Run(browser,
		chromedp.Poll(`performance.getEntriesByType("resource").filter(entry => entry.name.includes("/api/dashboard")).length === 2`, nil),
	); err != nil {
		t.Fatalf("settle the superseded request: %v", err)
	}
	var total string
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelector("#dashboard-stats strong").textContent`, &total)); err != nil {
		t.Fatalf("inspect final dashboard total: %v", err)
	}
	if total != "7" {
		t.Fatalf("dashboard total = %q, want the latest 7-day response", total)
	}
}

func TestDashboardOverlappingControlsClearSupersededBusyState(t *testing.T) {
	server, rangeStarted, releaseRange := newDashboardOverlapServer(t)
	browser := newBrowserContext(t, 20*time.Second)

	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#threat-actor-bars`),
		chromedp.Evaluate(`(() => {
			const select = document.querySelector("#dashboard-range");
			select.value = "7";
			select.dispatchEvent(new Event("change", { bubbles: true }));
		})()`, nil),
	); err != nil {
		t.Fatalf("start range refresh: %v", err)
	}
	waitForDashboardRequest(t, rangeStarted, "range dashboard request")

	if err := chromedp.Run(browser,
		chromedp.Click(`#hide-none-actor`),
		chromedp.Poll(`document.querySelector("#threat-actor-bars .bar-label")?.textContent === "LockBit"`, nil),
	); err != nil {
		t.Fatalf("complete actor refresh: %v", err)
	}
	releaseRange()

	var result map[string]int
	if err := chromedp.Run(browser,
		chromedp.Poll(`performance.getEntriesByType("resource").filter(entry => entry.name.includes("/api/dashboard")).length === 3`, nil),
		chromedp.Evaluate(`({ busyCount: document.querySelectorAll('[aria-busy="true"]').length, errorCount: document.querySelectorAll('.toast.is-error').length })`, &result),
	); err != nil {
		t.Fatalf("settle overlapping refreshes: %v", err)
	}
	if result["busyCount"] != 0 || result["errorCount"] != 0 {
		t.Fatalf("settled dashboard state = %+v, want no busy region or stale error", result)
	}
}

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
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{Settings: api.SettingsResponse{Language: "en"}})
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
		CVEs: []api.CVEInsight{{
			ID: "CVE-2026-1001", CVSS: 8.1, AffectedProduct: "acme / gateway", FirstSeen: "2026-08-01", Mentions: 2,
		}},
	}
}

func waitForDashboardRequest(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
