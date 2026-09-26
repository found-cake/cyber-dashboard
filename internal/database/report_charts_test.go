package database

import (
	"context"
	"path/filepath"
	"testing"

	sqlite "github.com/found-cake/gorm-sqlite"
	"gorm.io/gorm"
)

func TestOpenPreservesLegacyReport_whenChartSnapshotColumnIsAdded(t *testing.T) {
	// Given a report table from before chart snapshots were supported.
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := gorm.Open(sqlite.Open(dataSourceName(path)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE reports (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL,
		period_start TEXT NOT NULL, period_end TEXT NOT NULL, total INTEGER NOT NULL,
		critical INTEGER NOT NULL, high INTEGER NOT NULL, medium INTEGER NOT NULL,
		top_threat TEXT NOT NULL, top_threats TEXT NOT NULL DEFAULT '[]', actors TEXT NOT NULL,
		sectors TEXT NOT NULL, summary TEXT NOT NULL, generated_at TEXT NOT NULL)`,
		`INSERT INTO reports VALUES (1, 'weekly', '2026-08-01', '2026-08-07', 2, 0, 1, 1,
		'Incident', '[]', '[]', '[]', 'Legacy summary', '2026-08-08')`,
	} {
		if err := legacy.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := Close(legacy); err != nil {
		t.Fatal(err)
	}
	// When the existing application migration opens the database.
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(db); err != nil {
			t.Error(err)
		}
	})
	// Then the report survives and its absent chart history stays distinguishable.
	var report Report
	if err := db.First(&report, 1).Error; err != nil {
		t.Fatal(err)
	}
	if report.Charts != "" || report.Summary != "Legacy summary" || report.Total != 2 {
		t.Fatalf("migrated report = %+v", report)
	}
}
