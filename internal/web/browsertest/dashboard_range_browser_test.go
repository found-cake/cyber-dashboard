//go:build browser

package browsertest

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestDashboardControlsRepaintOnlyTheirOwnBlocks(t *testing.T) {
	server, requests := newDashboardPartialRenderServer(t)
	browser := newBrowserContext(t, 20*time.Second)
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
	if !toggled.CVETableKept || !toggled.StatsKept || !toggled.MethodBarsKept || !toggled.FocusKept || toggled.FirstActor != "LockBit" {
		t.Fatalf("None filter result = %+v", toggled)
	}

	var ranged struct {
		CVETableKept   bool `json:"cveTableKept"`
		StatsKept      bool `json:"statsKept"`
		MethodBarsKept bool `json:"methodBarsKept"`
		FocusKept      bool `json:"focusKept"`
	}
	if err := chromedp.Run(browser,
		chromedp.Evaluate(markPartialRenderProbes, nil),
		chromedp.Evaluate(`(() => {
			const select = document.querySelector("#dashboard-range");
			select.focus(); select.value = "7";
			select.dispatchEvent(new Event("change", { bubbles: true }));
		})()`, nil),
		chromedp.Poll(`document.querySelector("#threat-actors-card .card-subtitle")?.textContent.includes("7") === true`, nil),
		chromedp.Evaluate(readPartialRenderProbes, &ranged),
	); err != nil {
		t.Fatalf("change the aggregation range: %v", err)
	}
	if !ranged.CVETableKept || ranged.StatsKept || ranged.MethodBarsKept || !ranged.FocusKept {
		t.Fatalf("range refresh result = %+v", ranged)
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
	var final struct {
		Total   string `json:"total"`
		Buckets int    `json:"buckets"`
		Label   string `json:"label"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`({
		total: document.querySelector("#dashboard-stats strong").textContent,
		buckets: document.querySelectorAll("#collection-trend [data-bucket]").length,
		label: document.querySelector("#collection-trend [data-bucket]").getAttribute("aria-label")
	})`, &final)); err != nil {
		t.Fatalf("inspect final dashboard: %v", err)
	}
	if final.Total != "7" || final.Buckets != 7 || !strings.Contains(final.Label, "Total 6") {
		t.Fatalf("final dashboard = %+v, want the latest 7-day trend response", final)
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
