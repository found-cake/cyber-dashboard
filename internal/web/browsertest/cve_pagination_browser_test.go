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

	// Then all entries are rendered after the browser requests the three consecutive offsets.
	wantRequests := []string{
		"/api/cves?sort=score&offset=0",
		"/api/cves?sort=score&offset=100&revision=1",
		"/api/cves?sort=score&offset=200&revision=1",
	}
	if requests := server.cveRequests(); !slices.Equal(requests, wantRequests) {
		t.Fatalf("CVE requests = %v, want %v", requests, wantRequests)
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
		"/api/cves?sort=score&offset=0",
		"/api/cves?sort=score&offset=100&revision=1",
		"/api/cves?sort=score&offset=0",
		"/api/cves?sort=score&offset=100&revision=2",
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
	server.failNextPageAt(100)
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
