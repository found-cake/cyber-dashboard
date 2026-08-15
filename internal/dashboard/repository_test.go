package dashboard

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestDashboardAggregatesRecentThreatData(t *testing.T) {
	// Given recent articles with distinct severities, methods, actors, and one CVE.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	for _, row := range []struct{ uid, title, severity, method, actor string }{
		{uid: "critical", title: "Critical threat", severity: "CRITICAL", method: "APT", actor: "Group A"},
		{uid: "high", title: "High threat", severity: "HIGH", method: "Ransomware", actor: "Group B"},
	} {
		err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at, attack_method, threat_actor, severity)
      VALUES (1, ?, ?, 'https://example.com', ?, ?, ?, ?, ?)`, row.uid, row.title, today, today, row.method, row.actor, row.severity).Error
		if err != nil {
			t.Fatalf("insert article: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
    VALUES ('CVE-2026-1000', ?, 9.8, 'Example')`, today).Error; err != nil {
		t.Fatalf("insert CVE: %v", err)
	}
	if err := db.Exec(`INSERT INTO article_cves (article_id, cve_id)
    SELECT id, 'CVE-2026-1000' FROM articles WHERE feed_uid = 'critical'`).Error; err != nil {
		t.Fatalf("link CVE mention: %v", err)
	}

	// When the dashboard repository builds its response.
	value, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, false)
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}

	// Then totals and public breakdown arrays reflect the stored data.
	if value.Empty || value.Total != 2 || value.Critical != 1 || value.High != 1 || value.CVECount != 1 {
		t.Fatalf("dashboard = %+v", value)
	}
	if len(value.AttackMethods) != 2 || len(value.ThreatActors) != 2 || len(value.CVEs) != 1 {
		t.Fatalf("dashboard collections = %+v", value)
	}
}

func TestDashboardExcludesNoneActorBeforeTheTopCut(t *testing.T) {
	// Given a "None" bucket larger than any named actor, and more named actors than slots.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	insertArticle := func(uid, actor string) {
		t.Helper()
		if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at, attack_method, threat_actor)
      VALUES (1, ?, 'Threat', 'https://example.com', ?, ?, 'Ransomware', ?)`, uid, today, today, actor).Error; err != nil {
			t.Fatalf("insert article %s: %v", uid, err)
		}
	}
	for index := range 3 {
		insertArticle(fmt.Sprintf("none-%d", index), "None")
	}
	for index := range 9 {
		insertArticle(fmt.Sprintf("actor-%d", index), fmt.Sprintf("Group %d", index))
	}

	// When the caller hides the "None" bucket.
	value, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, true)
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}

	// Then all eight slots carry named actors rather than losing one to "None".
	if len(value.ThreatActors) != 8 {
		t.Fatalf("threat actors = %+v, want 8 rows", value.ThreatActors)
	}
	for _, row := range value.ThreatActors {
		if row.Label == "None" {
			t.Fatalf("threat actors = %+v, want no None bucket", value.ThreatActors)
		}
	}

	// And the unfiltered breakdown still reports it.
	kept, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, false)
	if err != nil {
		t.Fatalf("build unfiltered dashboard: %v", err)
	}
	if len(kept.ThreatActors) == 0 || kept.ThreatActors[0].Label != "None" {
		t.Fatalf("threat actors = %+v, want None ranked first", kept.ThreatActors)
	}
}

func TestDashboardCountsOnlyCVEsMentionedInsideTheWindow(t *testing.T) {
	// Given a CVE mentioned inside the window, one only before it, and one never mentioned.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	today := time.Now().Format(time.DateOnly)
	old := time.Now().AddDate(0, 0, -40).Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	for _, row := range []struct{ uid, day, cve string }{
		{uid: "recent", day: today, cve: "CVE-2026-1000"},
		{uid: "recent-again", day: today, cve: "CVE-2026-1000"},
		{uid: "stale", day: old, cve: "CVE-2026-2000"},
	} {
		if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
      VALUES (1, ?, 'Threat', 'https://example.com', ?, ?)`, row.uid, row.day, row.day).Error; err != nil {
			t.Fatalf("insert article %s: %v", row.uid, err)
		}
		if err := db.Exec(`INSERT OR IGNORE INTO cves (cve_id, first_seen, cvss_score, affected_product)
      VALUES (?, ?, 7.5, 'Example')`, row.cve, row.day).Error; err != nil {
			t.Fatalf("insert CVE %s: %v", row.cve, err)
		}
		if err := db.Exec(`INSERT INTO article_cves (article_id, cve_id)
      SELECT id, ? FROM articles WHERE feed_uid = ?`, row.cve, row.uid).Error; err != nil {
			t.Fatalf("link CVE for %s: %v", row.uid, err)
		}
	}
	if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
    VALUES ('CVE-2026-3000', ?, 6.1, 'Unmentioned')`, today).Error; err != nil {
		t.Fatalf("insert unmentioned CVE: %v", err)
	}

	// When the dashboard aggregates the window.
	value, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, false)
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}

	// Then the stat counts the one in-window CVE once, while the table keeps the full catalogue.
	if value.CVECount != 1 {
		t.Fatalf("cve count = %d, want 1", value.CVECount)
	}
	if len(value.CVEs) != 3 {
		t.Fatalf("cve rows = %d, want the complete catalogue of 3", len(value.CVEs))
	}
}

func TestBreakdownRejectsColumnOutsideAllowlist(t *testing.T) {
	// Given a repository and a column containing SQL syntax.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	// When the column reaches the dynamic breakdown boundary.
	_, err = NewRepository(db).breakdown(context.Background(), "attack_method); DROP TABLE articles; --", 8, "2026-07-06", false)

	// Then it is rejected before a SQL statement is selected.
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("breakdown error = %v, want invalid column", err)
	}
}

func TestDashboardRanksCVEsByCVSSPlusWeightedMentions(t *testing.T) {
	// Given one higher-CVSS CVE and one lower-CVSS CVE with more article mentions.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product) VALUES
    ('CVE-2026-9000', ?, 9.0, 'High CVSS'),
    ('CVE-2026-8000', ?, 8.0, 'More mentions')`, today, today).Error; err != nil {
		t.Fatalf("insert CVEs: %v", err)
	}
	for index := range 7 {
		result := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
      VALUES (1, ?, 'Threat', 'https://example.com', ?, ?)`, "mention-"+string(rune('a'+index)), today, today)
		if result.Error != nil {
			t.Fatalf("insert article: %v", result.Error)
		}
		var articleID int64
		if insertErr := db.Raw("SELECT last_insert_rowid()").Row().Scan(&articleID); insertErr != nil {
			t.Fatalf("article id: %v", insertErr)
		}
		if insertErr := db.Exec(`INSERT INTO article_cves (article_id, cve_id) VALUES (?, 'CVE-2026-8000')`, articleID).Error; insertErr != nil {
			t.Fatalf("link lower-CVSS CVE: %v", insertErr)
		}
	}
	result := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
    VALUES (1, 'high-cvss', 'Threat', 'https://example.com', ?, ?)`, today, today)
	if result.Error != nil {
		t.Fatalf("insert high-CVSS article: %v", result.Error)
	}
	var articleID int64
	if err := db.Raw("SELECT last_insert_rowid()").Row().Scan(&articleID); err != nil {
		t.Fatalf("high-CVSS article id: %v", err)
	}
	if err := db.Exec(`INSERT INTO article_cves (article_id, cve_id) VALUES (?, 'CVE-2026-9000')`, articleID).Error; err != nil {
		t.Fatalf("link high-CVSS CVE: %v", err)
	}

	// When the dashboard CVE insights are loaded.
	value, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, false)

	// Then the lower CVSS entry's 9.4 score precedes the higher entry's 9.2 score.
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}
	if len(value.CVEs) < 2 || value.CVEs[0].ID != "CVE-2026-8000" || value.CVEs[1].ID != "CVE-2026-9000" {
		t.Fatalf("CVE order = %+v", value.CVEs)
	}
}

func TestDashboardReturnsAllCVEsForExplorer(t *testing.T) {
	// Given more CVEs than the compact dashboard table displays.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	for index := range 10 {
		if err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
      VALUES (?, ?, ?, 'Example')`, fmt.Sprintf("CVE-2026-%04d", index), today, float64(index)).Error; err != nil {
			t.Fatalf("insert CVE %d: %v", index, err)
		}
	}

	// When the dashboard data is loaded for the compact table and explorer.
	value, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, false)

	// Then every ranked CVE is available to the explorer.
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}
	if len(value.CVEs) != 10 {
		t.Fatalf("CVE count = %d, want 10", len(value.CVEs))
	}
}
