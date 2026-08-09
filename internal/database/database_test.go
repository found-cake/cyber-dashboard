package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenAppliesSchemaAndSeedsIdempotently(t *testing.T) {
	// Given a new persistent database path.
	path := filepath.Join(t.TempDir(), "dashboard.db")

	// When the database is opened, closed, and opened again.
	for range 2 {
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		if err := Close(db); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })

	// Then source and built-in preset seeds exist exactly once.
	var sources, presets int
	if err := db.Raw(`SELECT COUNT(*) FROM sources`).Row().Scan(&sources); err != nil {
		t.Fatalf("count sources: %v", err)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM llm_presets WHERE builtin = 1`).Row().Scan(&presets); err != nil {
		t.Fatalf("count built-in presets: %v", err)
	}
	if sources != 6 || presets != 1 {
		t.Fatalf("seed counts = sources:%d presets:%d, want 6 and 1", sources, presets)
	}
}

func TestOpenEnforcesForeignKeys_whenPoolReplacesTheConnection(t *testing.T) {
	// Given an open database whose pooled connection is replaced.
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access database pool: %v", err)
	}
	sqlDB.SetMaxIdleConns(0)
	for range 3 {
		var probe int
		if err := db.Raw(`SELECT 1`).Row().Scan(&probe); err != nil {
			t.Fatalf("cycle connection: %v", err)
		}
	}

	// When a row that violates a foreign key is inserted on the replacement connection.
	err = db.Exec(`INSERT INTO article_cves (article_id, cve_id) VALUES (999999, 'CVE-0000-0000')`).Error

	// Then the constraint is still enforced: the pragma travels with every connection.
	if err == nil {
		t.Fatal("orphan article_cves row was accepted, want foreign key violation")
	}
	var enabled int
	if err := db.Raw(`PRAGMA foreign_keys`).Row().Scan(&enabled); err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
	var busyTimeout, synchronous int
	if err := db.Raw(`PRAGMA busy_timeout`).Row().Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout pragma: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
	var journalMode string
	if err := db.Raw(`PRAGMA journal_mode`).Row().Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode pragma: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	if err := db.Raw(`PRAGMA synchronous`).Row().Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous pragma: %v", err)
	}
	if synchronous != 2 {
		t.Fatalf("synchronous = %d, want FULL (2)", synchronous)
	}
}

func TestOpenInitializesLocalTimezoneAndPreservesSavedValue(t *testing.T) {
	// Given a new database and the current local timezone offset.
	path := filepath.Join(t.TempDir(), "dashboard.db")
	_, offsetSeconds := time.Now().Zone()

	// When the database is opened for the first time.
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	// Then the timezone is initialized from the local environment.
	var offsetMinutes int
	if err := db.Raw(`SELECT timezone_offset_minutes FROM settings WHERE id = 1`).Row().Scan(&offsetMinutes); err != nil {
		t.Fatalf("read initialized timezone: %v", err)
	}
	if want := offsetSeconds / 60; offsetMinutes != want {
		t.Fatalf("timezone offset = %d, want %d", offsetMinutes, want)
	}

	// When a user-defined timezone is saved and the database is reopened.
	const savedOffset = 123
	if err := db.Exec(`UPDATE settings SET timezone_offset_minutes = ? WHERE id = 1`, savedOffset).Error; err != nil {
		t.Fatalf("save timezone: %v", err)
	}
	if err := Close(db); err != nil {
		t.Fatalf("close database: %v", err)
	}
	db, err = Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })

	// Then the saved timezone is preserved.
	if err := db.Raw(`SELECT timezone_offset_minutes FROM settings WHERE id = 1`).Row().Scan(&offsetMinutes); err != nil {
		t.Fatalf("read saved timezone: %v", err)
	}
	if offsetMinutes != savedOffset {
		t.Fatalf("timezone offset = %d, want saved value %d", offsetMinutes, savedOffset)
	}
}

func TestOpenCreatesIndexesUsedByArticleDateAndCVEQueries(t *testing.T) {
	// Given a database initialized from the final schema.
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })

	tests := []struct {
		name      string
		statement string
		argument  string
		index     string
	}{
		{
			name: "articles by publication date",
			statement: `EXPLAIN QUERY PLAN SELECT id FROM articles
				WHERE published_at = ? ORDER BY published_time DESC, id DESC`,
			argument: "2026-08-04",
			index:    "articles_published_at_time_id_idx",
		},
		{
			name:      "article links by CVE",
			statement: `EXPLAIN QUERY PLAN SELECT article_id FROM article_cves WHERE cve_id = ?`,
			argument:  "CVE-2026-0001",
			index:     "article_cves_cve_id_article_id_idx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When SQLite plans the production query shape.
			rows, err := db.Raw(test.statement, test.argument).Rows()
			if err != nil {
				t.Fatalf("explain query plan: %v", err)
			}
			defer rows.Close()
			var details []string
			for rows.Next() {
				var id, parent, notUsed int
				var detail string
				if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
					t.Fatalf("scan query plan: %v", err)
				}
				details = append(details, detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("read query plan: %v", err)
			}

			// Then the matching composite index is selected.
			if plan := strings.Join(details, "\n"); !strings.Contains(plan, test.index) {
				t.Fatalf("query plan = %q, want index %q", plan, test.index)
			}
		})
	}
}
