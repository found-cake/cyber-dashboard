package database_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/database"
	_ "modernc.org/sqlite"
)

func TestOpenAddsLocalTimezoneOffset_whenExistingDatabasePredatesSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	_, err = legacy.Exec(`CREATE TABLE settings (
		id INTEGER PRIMARY KEY CHECK (id = 1), lang TEXT NOT NULL DEFAULT 'ko',
		theme TEXT NOT NULL DEFAULT 'dark', accent TEXT NOT NULL DEFAULT '#4f6ef7',
		llm_base_url TEXT NOT NULL DEFAULT 'https://api.openai.com/v1',
		llm_model TEXT NOT NULL DEFAULT 'gpt-4o-mini', llm_api_key TEXT NOT NULL DEFAULT '',
		llm_timeout INTEGER NOT NULL DEFAULT 60, nvd_api_key TEXT NOT NULL DEFAULT '');
		INSERT INTO settings (id) VALUES (1);`)
	if err != nil {
		t.Fatalf("create legacy settings: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	defer db.Close()
	var got int
	if err := db.QueryRow(`SELECT timezone_offset_minutes FROM settings WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("read timezone offset: %v", err)
	}
	_, seconds := time.Now().Zone()
	if got != seconds/60 {
		t.Fatalf("offset = %d, want local offset %d", got, seconds/60)
	}
}
