package dashboard

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestTrendFoldsDaysIntoFixedBucketsAcrossTheWholeWindow(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	start := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		uid      string
		offset   int
		severity string
		actor    string
	}{
		{uid: "first-bucket-a", offset: 0, severity: "CRITICAL", actor: "Lazarus Group"},
		{uid: "first-bucket-b", offset: 2, severity: "HIGH", actor: "Unknown"},
		{uid: "second-bucket", offset: 3, severity: "MEDIUM", actor: "Unknown (Russian-speaking)"},
		{uid: "no-actor", offset: 3, severity: "LOW", actor: "None"},
	} {
		day := start.AddDate(0, 0, row.offset).Format(time.DateOnly)
		if err := db.Exec(`INSERT INTO articles (source_id, feed_uid, title, url, published_at, collected_at, threat_actor, severity)
      VALUES (1, ?, 'Threat', 'https://example.com', ?, ?, ?, ?)`, row.uid, day, day, row.actor, row.severity).Error; err != nil {
			t.Fatalf("insert article %s: %v", row.uid, err)
		}
	}

	windowStart := start.Format(time.DateOnly)
	value, err := NewRepository(db).Dashboard(context.Background(), Window{Since: windowStart, Days: 30, Bucket: 3}, false)
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}

	if len(value.Trend) != 10 {
		t.Fatalf("trend points = %d, want 10", len(value.Trend))
	}
	first := value.Trend[0]
	if first.Start != windowStart || first.End != start.AddDate(0, 0, 2).Format(time.DateOnly) {
		t.Fatalf("first bucket span = %s..%s", first.Start, first.End)
	}
	if first.Total != 2 || first.Critical != 1 || first.High != 1 || first.Medium != 0 {
		t.Fatalf("first bucket volume = %+v", first)
	}
	if first.Attributed != 2 || first.NamedActor != 1 || first.UnknownActor != 1 || first.QualifiedUnknown != 0 {
		t.Fatalf("first bucket attribution = %+v", first)
	}
	second := value.Trend[1]
	if second.Total != 2 || second.Medium != 1 || second.Attributed != 1 || second.QualifiedUnknown != 1 || second.NamedActor != 0 {
		t.Fatalf("second bucket = %+v", second)
	}
	if value.Trend[9].Total != 0 {
		t.Fatalf("trailing bucket = %+v, want an empty slot", value.Trend[9])
	}
}

func TestTrendPointCountFollowsTheBucketSize(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository := NewRepository(db)
	today := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		window Window
		want   int
	}{
		{window: Window{Days: 7, Bucket: 1}, want: 7},
		{window: Window{Days: 30, Bucket: 3}, want: 10},
		{window: Window{Days: 90, Bucket: 9}, want: 10},
	} {
		test.window.Since = today.AddDate(0, 0, -(test.window.Days - 1)).Format(time.DateOnly)
		value, err := repository.Dashboard(context.Background(), test.window, false)
		if err != nil {
			t.Fatalf("build %d-day dashboard: %v", test.window.Days, err)
		}
		if len(value.Trend) != test.want {
			t.Fatalf("%d-day trend points = %d, want %d", test.window.Days, len(value.Trend), test.want)
		}
		if last := value.Trend[len(value.Trend)-1]; last.End != today.Format(time.DateOnly) {
			t.Fatalf("%d-day trend ends at %s, want today", test.window.Days, last.End)
		}
	}
}

func TestTrendRejectsAnUnevenWindow(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	_, err = NewRepository(db).Dashboard(context.Background(), Window{Since: "2026-08-01", Days: 8, Bucket: 3}, false)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("uneven trend window error = %v, want invalid", err)
	}
}
