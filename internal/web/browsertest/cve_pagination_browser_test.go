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
		"/api/cves?sort=score&offset=100",
		"/api/cves?sort=score&offset=200",
	}
	if requests := server.cveRequests(); !slices.Equal(requests, wantRequests) {
		t.Fatalf("CVE requests = %v, want %v", requests, wantRequests)
	}
}
