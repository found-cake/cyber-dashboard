package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	sqlite "github.com/found-cake/gorm-sqlite"
	"gorm.io/gorm"
)

func TestOpenBackfillsAndMaintainsCVEMentionRankingState_whenLegacyDatabaseMigrates(t *testing.T) {
	// Given a legacy database with one CVE mention and no materialized ranking fields.
	path := filepath.Join(t.TempDir(), "dashboard.db")
	legacy, err := gorm.Open(sqlite.Open(dataSourceName(path)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	statements := []string{
		`CREATE TABLE sources (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, host TEXT NOT NULL, slug TEXT NOT NULL UNIQUE, enabled NUMERIC NOT NULL)`,
		`CREATE TABLE cves (cve_id TEXT PRIMARY KEY, first_seen TEXT NOT NULL, cvss_score REAL NOT NULL DEFAULT 0,
      cvss_source TEXT NOT NULL DEFAULT '', cvss_version TEXT NOT NULL DEFAULT '', cvss_vector TEXT NOT NULL DEFAULT '',
      affected_product TEXT NOT NULL DEFAULT 'NVD enrichment pending')`,
		`CREATE TABLE articles (id INTEGER PRIMARY KEY AUTOINCREMENT, source_id INTEGER NOT NULL, feed_uid TEXT NOT NULL UNIQUE,
      title TEXT NOT NULL, url TEXT NOT NULL, published_at TEXT, published_time TEXT NOT NULL DEFAULT '', collected_at TEXT NOT NULL,
      body TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', attack_method TEXT NOT NULL DEFAULT 'Unclassified',
      threat_actor TEXT NOT NULL DEFAULT 'Unknown', actor_country TEXT NOT NULL DEFAULT '', sector TEXT NOT NULL DEFAULT '일반',
      victim_count INTEGER NOT NULL DEFAULT 0, damage_usd INTEGER NOT NULL DEFAULT 0, zero_day NUMERIC NOT NULL DEFAULT false,
      patch_available TEXT NOT NULL DEFAULT '', severity TEXT NOT NULL DEFAULT 'UNKNOWN')`,
		`CREATE TABLE article_cves (article_id INTEGER NOT NULL, cve_id TEXT NOT NULL, PRIMARY KEY (article_id, cve_id))`,
		`INSERT INTO sources (id, name, host, slug, enabled) VALUES (1, 'Legacy', 'example.com', 'legacy', true)`,
		`INSERT INTO cves (cve_id, first_seen) VALUES ('CVE-2026-0001', '2026-08-01')`,
		`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
      VALUES (1, 'legacy-one', 'Threat', 'https://example.com/one', '2026-08-01', '2026-08-01')`,
		`INSERT INTO article_cves (article_id, cve_id) VALUES (1, 'CVE-2026-0001')`,
	}
	for _, statement := range statements {
		if err := legacy.Exec(statement).Error; err != nil {
			t.Fatalf("prepare legacy database: %v", err)
		}
	}
	legacySQL, err := legacy.DB()
	if err != nil {
		t.Fatalf("access legacy database: %v", err)
	}
	if err := legacySQL.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	// When the database migrates and CVE links are inserted and deleted.
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })
	var migratedMentions int
	if err := db.Raw(`SELECT mention_count FROM cves WHERE cve_id = 'CVE-2026-0001'`).Row().Scan(&migratedMentions); err != nil {
		t.Fatalf("read migrated mention count: %v", err)
	}
	var revisionBefore uint64
	if err := db.Raw(`SELECT revision FROM cve_states WHERE id = 1`).Row().Scan(&revisionBefore); err != nil {
		t.Fatalf("read initial CVE revision: %v", err)
	}
	if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at)
      VALUES (1, 'legacy-two', 'Threat', 'https://example.com/two', '2026-08-01', '2026-08-01')`).Error; err != nil {
		t.Fatalf("insert second article: %v", err)
	}
	if err := db.Exec(`INSERT INTO article_cves (article_id, cve_id) VALUES (2, 'CVE-2026-0001')`).Error; err != nil {
		t.Fatalf("insert second CVE link: %v", err)
	}
	if err := db.Exec(`DELETE FROM article_cves WHERE article_id = 1 AND cve_id = 'CVE-2026-0001'`).Error; err != nil {
		t.Fatalf("delete first CVE link: %v", err)
	}

	// Then migration backfills the legacy mention, triggers keep the count correct, and the revision advances.
	var mentions int
	var riskKey, cvssKey float64
	var mentionsKey, firstSeenKey int
	if err := db.Raw(`SELECT mention_count, risk_key, cvss_key, mentions_key, first_seen_key
      FROM cves WHERE cve_id = 'CVE-2026-0001'`).Row().Scan(&mentions, &riskKey, &cvssKey, &mentionsKey, &firstSeenKey); err != nil {
		t.Fatalf("read maintained mention count: %v", err)
	}
	var revisionAfter uint64
	if err := db.Raw(`SELECT revision FROM cve_states WHERE id = 1`).Row().Scan(&revisionAfter); err != nil {
		t.Fatalf("read updated CVE revision: %v", err)
	}
	if migratedMentions != 1 || mentions != 1 || riskKey != -0.2 || cvssKey != 0 || mentionsKey != -1 || firstSeenKey != -20260801 || revisionAfter <= revisionBefore {
		t.Fatalf("ranking state = migrated %d, current %d, keys %g/%g/%d/%d, revisions %d -> %d",
			migratedMentions, mentions, riskKey, cvssKey, mentionsKey, firstSeenKey, revisionBefore, revisionAfter)
	}
}

func TestOpenCreatesIndexesUsedByEveryCVERanking(t *testing.T) {
	// Given a database initialized with materialized CVE rankings.
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })
	tests := []struct {
		name      string
		order     string
		seek      string
		arguments []any
		indexName string
	}{
		{
			name: "score", order: "risk_key, first_seen_key, cve_id",
			seek:      "(risk_key, first_seen_key, cve_id) > (?, ?, ?)",
			arguments: []any{-8.2, -20260801, "CVE-2026-0001"}, indexName: "cves_score_seek_idx",
		},
		{
			name: "CVSS", order: "cvss_key, risk_key, first_seen_key, cve_id",
			seek:      "(cvss_key, risk_key, first_seen_key, cve_id) > (?, ?, ?, ?)",
			arguments: []any{-8.0, -8.2, -20260801, "CVE-2026-0001"}, indexName: "cves_cvss_seek_idx",
		},
		{
			name: "mentions", order: "mentions_key, risk_key, first_seen_key, cve_id",
			seek:      "(mentions_key, risk_key, first_seen_key, cve_id) > (?, ?, ?, ?)",
			arguments: []any{-1, -8.2, -20260801, "CVE-2026-0001"}, indexName: "cves_mentions_seek_idx",
		},
		{
			name: "first seen", order: "first_seen_key, risk_key, cve_id",
			seek:      "(first_seen_key, risk_key, cve_id) > (?, ?, ?)",
			arguments: []any{-20260801, -8.2, "CVE-2026-0001"}, indexName: "cves_first_seen_seek_idx",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When SQLite plans one server-owned ranking query.
			rows, err := db.Raw(`EXPLAIN QUERY PLAN SELECT cve_id FROM cves WHERE `+test.seek+` ORDER BY `+test.order+` LIMIT 100`, test.arguments...).Rows()
			if err != nil {
				t.Fatalf("explain CVE ranking: %v", err)
			}
			defer rows.Close()
			var details []string
			for rows.Next() {
				var id, parent, notUsed int
				var detail string
				if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
					t.Fatalf("scan CVE query plan: %v", err)
				}
				details = append(details, detail)
			}

			// Then the matching ranking index satisfies the order without a temporary sort.
			plan := strings.Join(details, "\n")
			if !strings.Contains(plan, "SEARCH cves USING ") || !strings.Contains(plan, test.indexName) || strings.Contains(plan, "TEMP B-TREE") {
				t.Fatalf("query plan = %q, want a seek on %q without temp sort", plan, test.indexName)
			}
		})
	}
}
