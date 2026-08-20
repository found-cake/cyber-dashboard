//go:build browser

package browsertest

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestCVEExplorerLoadsEveryServerPage_whenMoreThanOneHundredEntriesExist(t *testing.T) {
	// Given 205 server-sorted CVEs exposed in pages of at most 100.
	cves := make([]api.CVEInsight, 205)
	for index := range cves {
		cves[index] = api.CVEInsight{
			ID: fmt.Sprintf("CVE-2026-%04d", index), CVSS: 9.8,
			AffectedProduct: "QA product", FirstSeen: "2026-08-01", Mentions: 1,
		}
	}
	server := newCVENavigationServer(t, cves)
	browser := newBrowserContext(t, 20*time.Second)

	// When the explorer opens and finishes loading every page.
	if err := chromedp.Run(browser,
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.Poll(`document.querySelectorAll(".cve-page-table tbody tr").length === 205`, nil),
	); err != nil {
		t.Fatalf("load paged CVE explorer: %v", err)
	}

	// Then all entries are rendered after the browser requests three bounded cursor pages.
	wantRequests := []string{
		"/api/cves?sort=score",
		"/api/cves?sort=score&cursor=score.CVE-2026-0099&revision=1",
		"/api/cves?sort=score&cursor=score.CVE-2026-0199&revision=1",
	}
	if requests := server.cveRequests(); !slices.Equal(requests, wantRequests) {
		t.Fatalf("CVE requests = %v, want %v", requests, wantRequests)
	}
	rendered := readCVEExplorerSnapshot(t, browser)
	for index := range cves {
		wantID := fmt.Sprintf("CVE-2026-%04d", index)
		wantRank := fmt.Sprintf("%d", index+1)
		if rendered.IDs[index] != wantID || rendered.Ranks[index] != wantRank {
			t.Fatalf("rendered row %d = rank %q, ID %q; want rank %q, ID %q", index, rendered.Ranks[index], rendered.IDs[index], wantRank, wantID)
		}
	}
}

func TestCVEExplorerRendersFirstServerPage_whileContinuationIsLoading(t *testing.T) {
	// Given 205 server-sorted CVEs whose second page remains in flight.
	cves := make([]api.CVEInsight, 205)
	for index := range cves {
		cves[index] = api.CVEInsight{
			ID: fmt.Sprintf("CVE-2026-%04d", index), CVSS: 9.8,
			AffectedProduct: "QA product", FirstSeen: "2026-08-01", Mentions: 1,
		}
	}
	server := newCVENavigationServer(t, cves)
	continuationStarted := server.pauseNextContinuation()
	browser := newBrowserContext(t, 20*time.Second)

	// When the browser has received the first page and requested the continuation.
	if err := chromedp.Run(browser, chromedp.Navigate(server.URL+"/#cves")); err != nil {
		t.Fatalf("open paged CVE explorer: %v", err)
	}
	waitForDashboardRequest(t, continuationStarted, "CVE continuation request")

	// Then the first 100 entries are already visible without waiting for every page.
	var rows int
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelectorAll(".cve-page-table tbody tr").length`, &rows)); err != nil {
		t.Fatalf("inspect progressive CVE rows: %v", err)
	}
	if rows != 100 {
		t.Fatalf("visible CVE rows while continuation loads = %d, want 100", rows)
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelector(".cve-page-table tbody tr").dataset.firstPageProbe = "true"`, nil)); err != nil {
		t.Fatalf("mark first CVE page: %v", err)
	}
	server.releasePausedContinuation()
	if err := chromedp.Run(browser, chromedp.Poll(`document.querySelectorAll(".cve-page-table tbody tr").length === 205`, nil)); err != nil {
		t.Fatalf("finish progressive CVE pages: %v", err)
	}
	var keptFirstPage int
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelectorAll('[data-first-page-probe="true"]').length`, &keptFirstPage)); err != nil {
		t.Fatalf("inspect first CVE page after append: %v", err)
	}
	if keptFirstPage != 1 {
		t.Fatalf("first-page DOM probes after continuations = %d, want 1", keptFirstPage)
	}
}

func TestCVEExplorerRestartsEveryPage_whenRankingRevisionChanges(t *testing.T) {
	// Given a ranking that changes while the browser requests its continuation.
	cves := make([]api.CVEInsight, 101)
	for index := range cves {
		cves[index] = api.CVEInsight{
			ID: fmt.Sprintf("CVE-2026-%04d", index), CVSS: 8,
			AffectedProduct: "QA product", FirstSeen: "2026-08-01", Mentions: 1,
		}
	}
	server := newCVENavigationServer(t, cves)
	server.staleNextContinuation()
	browser := newBrowserContext(t, 20*time.Second)

	// When the continuation is rejected as stale.
	if err := chromedp.Run(browser,
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.Poll(`document.querySelectorAll(".cve-page-table tbody tr").length === 101`, nil),
	); err != nil {
		t.Fatalf("restart stale CVE explorer: %v", err)
	}

	// Then the browser discards the partial ranking and reloads every unique row at the new revision.
	var uniqueRows int
	if err := chromedp.Run(browser, chromedp.Evaluate(`new Set(Array.from(
		document.querySelectorAll(".cve-page-table tbody tr td:nth-child(2)"), cell => cell.textContent
	)).size`, &uniqueRows)); err != nil {
		t.Fatalf("inspect restarted CVE rows: %v", err)
	}
	if uniqueRows != len(cves) {
		t.Fatalf("unique CVE rows = %d, want %d", uniqueRows, len(cves))
	}
	wantRequests := []string{
		"/api/cves?sort=score",
		"/api/cves?sort=score&cursor=score.CVE-2026-0099&revision=1",
		"/api/cves?sort=score",
		"/api/cves?sort=score&cursor=score.CVE-2026-0098&revision=2",
	}
	if requests := server.cveRequests(); !slices.Equal(requests, wantRequests) {
		t.Fatalf("CVE requests = %v, want %v", requests, wantRequests)
	}
}

func TestCVEExplorerOffersRetry_whenLaterPageFails(t *testing.T) {
	// Given an explorer whose second page fails once.
	cves := make([]api.CVEInsight, 105)
	for index := range cves {
		cves[index] = api.CVEInsight{
			ID: fmt.Sprintf("CVE-2026-%04d", index), CVSS: 8,
			AffectedProduct: "QA product", FirstSeen: "2026-08-01", Mentions: 1,
		}
	}
	server := newCVENavigationServer(t, cves)
	server.failNextContinuation()
	browser := newBrowserContext(t, 20*time.Second)

	// When loading stops after the partial page.
	if err := chromedp.Run(browser,
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.WaitVisible("#retry-cves"),
	); err != nil {
		t.Fatalf("wait for CVE retry state: %v", err)
	}
	var skeletons int
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelectorAll("#main-content .skeleton").length`, &skeletons)); err != nil {
		t.Fatalf("inspect failed CVE state: %v", err)
	}
	if skeletons != 0 {
		t.Fatalf("loading skeletons = %d after failure, want none", skeletons)
	}

	// Then retry starts a complete replacement load and renders every row.
	if err := chromedp.Run(browser,
		chromedp.Click("#retry-cves"),
		chromedp.Poll(`document.querySelectorAll(".cve-page-table tbody tr").length === 105`, nil),
	); err != nil {
		t.Fatalf("retry CVE pages: %v", err)
	}
}
