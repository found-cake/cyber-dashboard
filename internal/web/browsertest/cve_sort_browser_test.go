//go:build browser

package browsertest

import (
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestCVEExplorerRequestsServerRanking_whenSortCriterionChanges(t *testing.T) {
	// Given CVEs whose risk-score order differs from their mention and first-seen order.
	cves := []api.CVEInsight{
		{ID: "CVE-2026-0001", CVSS: 9.8, AffectedProduct: "QA product", FirstSeen: "2026-08-01", Mentions: 1},
		{ID: "CVE-2026-0002", CVSS: 7.0, AffectedProduct: "QA product", FirstSeen: "2026-08-10", Mentions: 12},
		{ID: "CVE-2026-0003", CVSS: 5.0, AffectedProduct: "QA product", FirstSeen: "2026-08-15", Mentions: 2},
	}
	server := newCVENavigationServer(t, cves)
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser,
		chromedp.Navigate(server.URL+"/#cves"),
		chromedp.WaitVisible("#cve-sort"),
	); err != nil {
		t.Fatalf("open CVE explorer: %v", err)
	}

	selectSort := func(value, expected string) string {
		t.Helper()
		requestCount := len(server.cveRequests())
		var result struct {
			First     string `json:"first"`
			Hint      string `json:"hint"`
			FocusKept bool   `json:"focusKept"`
		}
		if err := chromedp.Run(browser,
			chromedp.Evaluate(`(() => {
				const select = document.querySelector("#cve-sort");
				select.focus(); select.value = "`+value+`";
				select.dispatchEvent(new Event("change", { bubbles: true }));
			})()`, nil),
			chromedp.Poll(`document.querySelector(".cve-page-table tbody tr td:nth-child(2)").textContent === "`+expected+`"`, nil),
			chromedp.Evaluate(`({
				first: document.querySelector(".cve-page-table tbody tr td:nth-child(2)").textContent,
				hint: document.querySelector("#cve-sort-hint").textContent,
				focusKept: document.activeElement === document.querySelector("#cve-sort")
			})`, &result),
		); err != nil {
			t.Fatalf("sort by %s: %v", value, err)
		}
		if !result.FocusKept {
			t.Fatalf("sorting by %s moved focus away from the select", value)
		}
		if result.Hint == "" {
			t.Fatalf("sorting by %s left the criterion hint empty", value)
		}
		requests := server.cveRequests()
		if len(requests) <= requestCount || requests[len(requests)-1] != "/api/cves?sort="+value+"&offset=0" {
			t.Fatalf("requests after sorting by %s = %v", value, requests)
		}
		return result.First
	}

	// When each criterion is selected, then the ranking follows that column.
	if first := selectSort("mentions", "CVE-2026-0002"); first != "CVE-2026-0002" {
		t.Fatalf("top row by mentions = %q, want CVE-2026-0002", first)
	}
	if first := selectSort("firstSeen", "CVE-2026-0003"); first != "CVE-2026-0003" {
		t.Fatalf("top row by first seen = %q, want CVE-2026-0003", first)
	}
	if first := selectSort("score", "CVE-2026-0001"); first != "CVE-2026-0001" {
		t.Fatalf("top row by risk score = %q, want CVE-2026-0001", first)
	}

	// Then the choice survives a return trip to the dashboard.
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`(() => {
			const select = document.querySelector("#cve-sort");
			select.value = "mentions";
			select.dispatchEvent(new Event("change", { bubbles: true }));
		})()`, nil),
		chromedp.Click(".cve-back-link"),
		chromedp.WaitVisible("#open-cve-explorer"),
		chromedp.Click("#open-cve-explorer"),
		chromedp.WaitVisible("#cve-sort"),
	); err != nil {
		t.Fatalf("revisit CVE explorer: %v", err)
	}
	var revisited struct {
		Value string `json:"value"`
		First string `json:"first"`
	}
	if err := chromedp.Run(browser, chromedp.Evaluate(`({
		value: document.querySelector("#cve-sort").value,
		first: document.querySelector(".cve-page-table tbody tr td:nth-child(2)").textContent
	})`, &revisited)); err != nil {
		t.Fatalf("inspect revisited explorer: %v", err)
	}
	if revisited.Value != "mentions" || revisited.First != "CVE-2026-0002" {
		t.Fatalf("revisited explorer = %+v, want the mention ranking kept", revisited)
	}
}
