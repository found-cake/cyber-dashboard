package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	// foreign_keys is a per-connection pragma, so it has to travel in the DSN: running it
	// once as a statement only arms the connection that executed it, and any replacement
	// connection the pool opens would silently stop enforcing the constraints.
	db, err := sql.Open("sqlite", dataSourceName(path))
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

// dataSourceName builds a file: DSN carrying the per-connection pragmas. busy_timeout
// keeps a contended write waiting instead of failing immediately with SQLITE_BUSY.
func dataSourceName(path string) string {
	return (&url.URL{
		Scheme:   "file",
		Opaque:   (&url.URL{Path: path}).EscapedPath(),
		RawQuery: url.Values{"_pragma": {"foreign_keys(1)", "busy_timeout(5000)"}}.Encode(),
	}).String()
}
