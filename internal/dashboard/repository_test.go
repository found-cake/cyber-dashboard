package dashboard

import (
	"context"
	"path/filepath"
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
	value, err := NewRepository(db).Dashboard(context.Background())
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
