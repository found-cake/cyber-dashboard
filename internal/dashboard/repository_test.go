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
	t.Cleanup(func() { _ = db.Close() })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	for _, row := range []struct{ uid, title, severity, method, actor string }{
		{uid: "critical", title: "Critical threat", severity: "CRITICAL", method: "APT", actor: "Group A"},
		{uid: "high", title: "High threat", severity: "HIGH", method: "Ransomware", actor: "Group B"},
	} {
		_, err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at, attack_method, threat_actor, severity)
      VALUES (1, ?, ?, 'https://example.com', ?, ?, ?, ?, ?)`, row.uid, row.title, today, today, row.method, row.actor, row.severity)
		if err != nil {
			t.Fatalf("insert article: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
    VALUES ('CVE-2026-1000', ?, 9.8, 'Example')`, today); err != nil {
		t.Fatalf("insert CVE: %v", err)
	}

	// When the dashboard repository builds its response.
	value, err := NewRepository(db).Dashboard(context.Background(), windowStart)
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

func TestBreakdownRejectsColumnOutsideAllowlist(t *testing.T) {
	// Given a repository and a column containing SQL syntax.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// When the column reaches the dynamic breakdown boundary.
	_, err = NewRepository(db).breakdown(context.Background(), "attack_method); DROP TABLE articles; --", 8, "2026-07-06")

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
	t.Cleanup(func() { _ = db.Close() })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	if _, err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product) VALUES
    ('CVE-2026-9000', ?, 9.0, 'High CVSS'),
    ('CVE-2026-8000', ?, 8.0, 'More mentions')`, today, today); err != nil {
		t.Fatalf("insert CVEs: %v", err)
	}
	for index := range 7 {
		result, insertErr := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
      VALUES (1, ?, 'Threat', 'https://example.com', ?, ?)`, "mention-"+string(rune('a'+index)), today, today)
		if insertErr != nil {
			t.Fatalf("insert article: %v", insertErr)
		}
		articleID, insertErr := result.LastInsertId()
		if insertErr != nil {
			t.Fatalf("article id: %v", insertErr)
		}
		if _, insertErr := db.Exec(`INSERT INTO article_cves (article_id, cve_id) VALUES (?, 'CVE-2026-8000')`, articleID); insertErr != nil {
			t.Fatalf("link lower-CVSS CVE: %v", insertErr)
		}
	}
	result, err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
    VALUES (1, 'high-cvss', 'Threat', 'https://example.com', ?, ?)`, today, today)
	if err != nil {
		t.Fatalf("insert high-CVSS article: %v", err)
	}
	articleID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("high-CVSS article id: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO article_cves (article_id, cve_id) VALUES (?, 'CVE-2026-9000')`, articleID); err != nil {
		t.Fatalf("link high-CVSS CVE: %v", err)
	}

	// When the dashboard CVE insights are loaded.
	value, err := NewRepository(db).Dashboard(context.Background(), windowStart)

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
	t.Cleanup(func() { _ = db.Close() })
	today := time.Now().Format(time.DateOnly)
	windowStart := time.Now().AddDate(0, 0, -29).Format(time.DateOnly)
	for index := range 10 {
		if _, err := db.Exec(`INSERT INTO cves (cve_id, first_seen, cvss_score, affected_product)
      VALUES (?, ?, ?, 'Example')`, fmt.Sprintf("CVE-2026-%04d", index), today, float64(index)); err != nil {
			t.Fatalf("insert CVE %d: %v", index, err)
		}
	}

	// When the dashboard data is loaded for the compact table and explorer.
	value, err := NewRepository(db).Dashboard(context.Background(), windowStart)

	// Then every ranked CVE is available to the explorer.
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}
	if len(value.CVEs) != 10 {
		t.Fatalf("CVE count = %d, want 10", len(value.CVEs))
	}
}
