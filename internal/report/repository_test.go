package report

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestBuildReturnsNotFoundForEmptyPeriod(t *testing.T) {
	// Given a new database with no articles.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// When a report is built for an empty period.
	_, _, err = NewRepository(db).Build(context.Background(), api.CreateReportRequest{
		Type: "weekly", Start: "2026-08-01", End: "2026-08-03",
	})

	// Then callers can map the stable not-found error to HTTP 404.
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("build error = %v, want ErrNotFound", err)
	}
}
