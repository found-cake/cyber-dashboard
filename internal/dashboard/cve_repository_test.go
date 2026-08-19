package dashboard

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestCVEInsightsReturnsFixedPages_withoutDroppingEntries(t *testing.T) {
	// Given more CVEs than two fixed-size API pages can hold.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	for index := range 205 {
		if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
      VALUES (?, '2026-08-01', 9.8, 'Example')`, fmt.Sprintf("CVE-2026-%04d", index)).Error; err != nil {
			t.Fatalf("insert CVE %d: %v", index, err)
		}
	}
	repository := NewRepository(db)

	// When each consecutive page is loaded.
	for _, sort := range []CVESort{CVESortScore, CVESortCVSS, CVESortMentions, CVESortFirstSeen} {
		t.Run(string(sort), func(t *testing.T) {
			first, err := repository.CVEInsights(context.Background(), CVEPageRequest{Sort: sort})
			if err != nil {
				t.Fatalf("load first CVE page: %v", err)
			}
			revision := first.Revision
			second, err := repository.CVEInsights(context.Background(), CVEPageRequest{Sort: sort, Cursor: first.NextCursor, ExpectedRevision: &revision})
			if err != nil {
				t.Fatalf("load second CVE page: %v", err)
			}
			third, err := repository.CVEInsights(context.Background(), CVEPageRequest{Sort: sort, Cursor: second.NextCursor, ExpectedRevision: &revision})
			if err != nil {
				t.Fatalf("load third CVE page: %v", err)
			}

			// Then every response is bounded and the pages form one complete ordered list.
			if len(first.Values) != CVEPageSize || len(second.Values) != CVEPageSize || len(third.Values) != 5 {
				t.Fatalf("page lengths = %d, %d, %d", len(first.Values), len(second.Values), len(third.Values))
			}
			if first.Values[0].ID != "CVE-2026-0000" || second.Values[0].ID != "CVE-2026-0100" || third.Values[0].ID != "CVE-2026-0200" {
				t.Fatalf("page starts = %q, %q, %q", first.Values[0].ID, second.Values[0].ID, third.Values[0].ID)
			}
		})
	}
}

func TestCVEInsightsSortsOnTheServer_forEveryExplorerCriterion(t *testing.T) {
	// Given CVEs whose risk, CVSS, mention, and first-seen rankings differ.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	fixtures := []struct {
		id        string
		cvss      float64
		firstSeen string
		mentions  int
	}{
		{id: "CVE-2026-0001", cvss: 9.8, firstSeen: "2026-08-01", mentions: 1},
		{id: "CVE-2026-0002", cvss: 7.0, firstSeen: "2026-08-10", mentions: 12},
		{id: "CVE-2026-0003", cvss: 5.0, firstSeen: "2026-08-15", mentions: 2},
		{id: "CVE-2026-0004", cvss: 9.8, firstSeen: "2026-07-01", mentions: 2},
	}
	for _, fixture := range fixtures {
		if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
      VALUES (?, ?, ?, 'Example')`, fixture.id, fixture.firstSeen, fixture.cvss).Error; err != nil {
			t.Fatalf("insert %s: %v", fixture.id, err)
		}
		for mention := range fixture.mentions {
			uid := fmt.Sprintf("%s-%d", fixture.id, mention)
			if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
          VALUES (1, ?, 'Threat', 'https://example.com', '2026-08-01', '2026-08-01')`, uid).Error; err != nil {
				t.Fatalf("insert article %s: %v", uid, err)
			}
			if err := db.Exec(`INSERT INTO article_cves (article_id, cve_id)
          SELECT id, ? FROM articles WHERE feed_uid = ?`, fixture.id, uid).Error; err != nil {
				t.Fatalf("link article %s: %v", uid, err)
			}
		}
	}
	repository := NewRepository(db)
	tests := []struct {
		name string
		sort CVESort
		want []string
	}{
		{name: "risk score", sort: CVESortScore, want: []string{"CVE-2026-0004", "CVE-2026-0001", "CVE-2026-0002", "CVE-2026-0003"}},
		{name: "CVSS", sort: CVESortCVSS, want: []string{"CVE-2026-0004", "CVE-2026-0001", "CVE-2026-0002", "CVE-2026-0003"}},
		{name: "mentions", sort: CVESortMentions, want: []string{"CVE-2026-0002", "CVE-2026-0004", "CVE-2026-0003", "CVE-2026-0001"}},
		{name: "first seen", sort: CVESortFirstSeen, want: []string{"CVE-2026-0003", "CVE-2026-0002", "CVE-2026-0001", "CVE-2026-0004"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When one explorer sort is requested.
			page, err := repository.CVEInsights(context.Background(), CVEPageRequest{Sort: test.sort})
			if err != nil {
				t.Fatalf("load sorted CVEs: %v", err)
			}

			// Then the repository returns the complete deterministic server ranking.
			ids := make([]string, len(page.Values))
			for index, value := range page.Values {
				ids[index] = value.ID
			}
			if !slices.Equal(ids, test.want) {
				t.Fatalf("order = %v, want %v", ids, test.want)
			}
		})
	}
}

func TestCVEInsightsRejectsContinuation_whenRankingRevisionChanges(t *testing.T) {
	// Given a first page and its ranking revision.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	for index := range CVEPageSize + 1 {
		if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
      VALUES (?, '2026-08-01', 5.0, 'Example')`, fmt.Sprintf("CVE-2026-%04d", index)).Error; err != nil {
			t.Fatalf("insert CVE %d: %v", index, err)
		}
	}
	repository := NewRepository(db)
	first, err := repository.CVEInsights(context.Background(), CVEPageRequest{Sort: CVESortScore})
	if err != nil {
		t.Fatalf("load first CVE page: %v", err)
	}
	if err := db.Exec(`UPDATE cves SET cvss_score = 10 WHERE cve_id = 'CVE-2026-0100'`).Error; err != nil {
		t.Fatalf("change CVE ranking: %v", err)
	}

	// When the next page is requested with the superseded revision.
	revision := first.Revision
	_, err = repository.CVEInsights(context.Background(), CVEPageRequest{
		Sort: CVESortScore, Cursor: first.NextCursor, ExpectedRevision: &revision,
	})

	// Then the repository rejects the inconsistent continuation instead of returning duplicates or omissions.
	if !errors.Is(err, ErrCVEPageStale) {
		t.Fatalf("continuation error = %v, want ErrCVEPageStale", err)
	}
}
