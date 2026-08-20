//go:build browser

package browsertest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

const observeOldSortCompletion = `(() => {
	window.__oldSortCompleted = false;
	$(document).off("ajaxComplete.cveOldSort").on("ajaxComplete.cveOldSort", (_event, _xhr, settings) => {
		if (settings.url.includes("sort=score") && settings.url.includes("cursor=")) window.__oldSortCompleted = true;
	});
})()`

type cveExplorerSnapshot struct {
	IDs          []string `json:"ids"`
	Ranks        []string `json:"ranks"`
	RetryVisible bool     `json:"retryVisible"`
	ErrorToasts  int      `json:"errorToasts"`
}

func TestCVEExplorerIgnoresOldSortSuccess_whenSortChangesDuringContinuation(t *testing.T) {
	// Given a score continuation that remains in flight while the CVSS ranking can complete.
	server := newCVENavigationServer(t, cveSortRaceFixture())
	continuationStarted := server.pauseNextContinuation()
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser, chromedp.Navigate(server.URL+"/#cves")); err != nil {
		t.Fatalf("open score-ranked CVE explorer: %v", err)
	}
	waitForDashboardRequest(t, continuationStarted, "score continuation request")

	// When the sort changes before the old continuation succeeds.
	selectCVESort(t, browser, "cvss")
	if err := chromedp.Run(browser,
		chromedp.Poll(`document.querySelectorAll(".cve-page-table tbody tr").length === 205`, nil),
		chromedp.Evaluate(observeOldSortCompletion, nil),
	); err != nil {
		t.Fatalf("load CVSS ranking before old score response: %v", err)
	}
	want := readCVEExplorerSnapshot(t, browser)
	server.releasePausedContinuation()
	waitForOldSortCompletion(t, browser)

	// Then the delayed score page cannot change the completed CVSS ranking.
	assertCVEExplorerSnapshot(t, readCVEExplorerSnapshot(t, browser), want)
	for _, request := range server.cveRequests() {
		if strings.Contains(request, "sort=score&cursor=") && strings.Contains(request, "CVE-2026-0199") {
			t.Fatalf("superseded score ranking requested another continuation: %s", request)
		}
	}
}

func TestCVEExplorerIgnoresOldSortFailure_whenSortChangesDuringContinuation(t *testing.T) {
	// Given a score continuation that will fail after the CVSS ranking completes.
	server := newCVENavigationServer(t, cveSortRaceFixture())
	continuationStarted := server.pauseNextContinuationFailure()
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser, chromedp.Navigate(server.URL+"/#cves")); err != nil {
		t.Fatalf("open score-ranked CVE explorer: %v", err)
	}
	waitForDashboardRequest(t, continuationStarted, "score continuation request")

	// When the sort changes before the old continuation fails.
	selectCVESort(t, browser, "cvss")
	if err := chromedp.Run(browser,
		chromedp.Poll(`document.querySelectorAll(".cve-page-table tbody tr").length === 205`, nil),
		chromedp.Evaluate(observeOldSortCompletion, nil),
	); err != nil {
		t.Fatalf("load CVSS ranking before old score failure: %v", err)
	}
	want := readCVEExplorerSnapshot(t, browser)
	server.releasePausedContinuation()
	waitForOldSortCompletion(t, browser)

	// Then the delayed failure cannot replace the ranking or surface an error.
	got := readCVEExplorerSnapshot(t, browser)
	assertCVEExplorerSnapshot(t, got, want)
	if got.RetryVisible || got.ErrorToasts != 0 {
		t.Fatalf("superseded score failure left retry=%t, error toasts=%d", got.RetryVisible, got.ErrorToasts)
	}
}

func cveSortRaceFixture() []api.CVEInsight {
	values := make([]api.CVEInsight, 205)
	for index := range values {
		values[index] = api.CVEInsight{
			ID: fmt.Sprintf("CVE-2026-%04d", index), CVSS: 10,
			AffectedProduct: "QA product", FirstSeen: "2026-08-01", Mentions: 1,
		}
		if index < 100 {
			values[index].CVSS = 1
			values[index].Mentions = 1000 - index
		}
	}
	return values
}

func selectCVESort(t *testing.T, browser context.Context, value string) {
	t.Helper()
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		const select = document.querySelector("#cve-sort");
		select.value = "`+value+`";
		select.dispatchEvent(new Event("change", { bubbles: true }));
	})()`, nil)); err != nil {
		t.Fatalf("select CVE sort %s: %v", value, err)
	}
}

func waitForOldSortCompletion(t *testing.T, browser context.Context) {
	t.Helper()
	if err := chromedp.Run(browser, chromedp.Poll(`window.__oldSortCompleted === true`, nil)); err != nil {
		t.Fatalf("wait for old sort request completion: %v", err)
	}
}

func readCVEExplorerSnapshot(t *testing.T, browser context.Context) cveExplorerSnapshot {
	t.Helper()
	var snapshot cveExplorerSnapshot
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		const rows = Array.from(document.querySelectorAll(".cve-page-table tbody tr"));
		return {
			ids: rows.map(row => row.querySelector("td:nth-child(2)").textContent),
			ranks: rows.map(row => row.querySelector("td:first-child").textContent),
			retryVisible: Boolean(document.querySelector("#retry-cves")),
			errorToasts: document.querySelectorAll("#toast-region .is-error").length
		};
	})()`, &snapshot)); err != nil {
		t.Fatalf("read CVE explorer snapshot: %v", err)
	}
	return snapshot
}

func assertCVEExplorerSnapshot(t *testing.T, got, want cveExplorerSnapshot) {
	t.Helper()
	if !slices.Equal(got.IDs, want.IDs) || !slices.Equal(got.Ranks, want.Ranks) ||
		got.RetryVisible != want.RetryVisible || got.ErrorToasts != want.ErrorToasts {
		t.Fatalf("CVE explorer changed after superseded response:\n got %+v\nwant %+v", got, want)
	}
}
