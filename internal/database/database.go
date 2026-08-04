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
	_, offsetSeconds := time.Now().Zone()
	if _, err := db.ExecContext(ctx, `UPDATE settings SET timezone_offset_minutes = ? WHERE timezone_offset_minutes IS NULL`, offsetSeconds/60); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize timezone setting: %w", err)
	}
	return db, nil
}
