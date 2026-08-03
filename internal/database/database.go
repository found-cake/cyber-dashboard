package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrateSchema(ctx, db, localTimezoneOffsetMinutes(time.Now())); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrateSchema(ctx context.Context, db *sql.DB, offsetMinutes int) error {
	columns := []struct {
		table      string
		name       string
		definition string
	}{
		{table: "settings", name: "timezone_offset_minutes", definition: "INTEGER"},
		{table: "articles", name: "published_time", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "articles", name: "victim_count", definition: "INTEGER NOT NULL DEFAULT 0"},
		{table: "articles", name: "zero_day", definition: "INTEGER NOT NULL DEFAULT 0"},
		{table: "cves", name: "cvss_source", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "cves", name: "cvss_version", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "llm_presets", name: "api_key", definition: "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		if err := ensureColumn(ctx, db, column.table, column.name, column.definition); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE settings SET timezone_offset_minutes = ? WHERE timezone_offset_minutes IS NULL`, offsetMinutes); err != nil {
		return fmt.Errorf("initialize timezone setting: %w", err)
	}
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version < 2 {
		if _, err := db.ExecContext(ctx, `UPDATE articles SET severity = 'UNKNOWN' WHERE severity = 'MEDIUM'`); err != nil {
			return fmt.Errorf("migrate default article severity: %w", err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE llm_presets SET api_key =
			(SELECT llm_api_key FROM settings WHERE id = 1)
			WHERE api_key = '' AND base_url = (SELECT llm_base_url FROM settings WHERE id = 1)
			AND model = (SELECT llm_model FROM settings WHERE id = 1)`); err != nil {
			return fmt.Errorf("migrate active LLM credential: %w", err)
		}
		if _, err := db.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("inspect %s schema: %w", table, err)
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan settings schema: %w", err)
		}
		found = found || name == column
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate %s schema: %w", table, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close %s schema: %w", table, err)
	}
	if !found {
		if _, err := db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
			return fmt.Errorf("add %s.%s: %w", table, column, err)
		}
	}
	return nil
}

func localTimezoneOffsetMinutes(now time.Time) int {
	_, seconds := now.Zone()
	return seconds / 60
}
