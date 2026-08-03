package database

import (
	"context"
	"path/filepath"
	"testing"
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
		if err := db.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Then source and built-in preset seeds exist exactly once.
	var sources, presets int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sources`).Scan(&sources); err != nil {
		t.Fatalf("count sources: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM llm_presets WHERE builtin = 1`).Scan(&presets); err != nil {
		t.Fatalf("count built-in presets: %v", err)
	}
	if sources != 6 || presets != 1 {
		t.Fatalf("seed counts = sources:%d presets:%d, want 6 and 1", sources, presets)
	}
}
