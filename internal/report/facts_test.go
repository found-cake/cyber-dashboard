package report

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestBuildUsesDailySummaries_whenReportIsWeeklyOrMonthly(t *testing.T) {
	// Given ten active days with stored daily summaries and many underlying articles.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	for day := 1; day <= 10; day++ {
		date := fmt.Sprintf("2026-08-%02d", day)
		if err := db.Create(&database.DailySummary{Day: date, Summary: "Daily digest " + date, GeneratedAt: date + "T23:00:00Z"}).Error; err != nil {
			t.Fatalf("insert daily summary %s: %v", date, err)
		}
		for item := 1; item <= 7; item++ {
			article := database.Article{
				SourceID: 1, FeedUID: fmt.Sprintf("%s-%d", date, item),
				Title: fmt.Sprintf("Incident %s item %d", date, item), URL: fmt.Sprintf("https://example.com/%s/%d", date, item),
				PublishedAt: date, CollectedAt: date + "T00:00:00Z", Summary: "Report evidence", Severity: "HIGH",
			}
			if err := db.Create(&article).Error; err != nil {
				t.Fatalf("insert article %s: %v", article.FeedUID, err)
			}
		}
	}
	repository := NewRepository(db)
	request := api.CreateReportRequest{Start: "2026-08-01", End: "2026-08-10"}

	// When the same period is prepared as weekly and monthly reports.
	request.Type = "weekly"
	weekly, weeklyErr := repository.Build(context.Background(), request)
	request.Type = "monthly"
	monthly, monthlyErr := repository.Build(context.Background(), request)

	// Then both reports use the ten daily summaries and never resample article summaries.
	if weeklyErr != nil || monthlyErr != nil {
		t.Fatalf("build weekly = %v, monthly = %v", weeklyErr, monthlyErr)
	}
	if len(weekly.facts) != 10 || len(monthly.facts) != 10 {
		t.Fatalf("fact counts weekly=%d monthly=%d, want 10 daily summaries", len(weekly.facts), len(monthly.facts))
	}
	for _, facts := range [][]string{weekly.facts, monthly.facts} {
		joined := strings.Join(facts, "\n")
		if !strings.Contains(joined, "Daily digest 2026-08-01") || strings.Contains(joined, "Report evidence") {
			t.Fatalf("report facts do not use only daily summaries: %q", joined)
		}
	}
}

func TestBuildRejectsPeriodicReport_whenAnActiveDayHasNoDailySummary(t *testing.T) {
	// Given two article days but a daily summary for only one of them.
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	for day := 1; day <= 2; day++ {
		date := fmt.Sprintf("2026-08-%02d", day)
		if err := db.Create(&database.Article{
			SourceID: 1, FeedUID: "missing-summary-" + date, Title: "Incident " + date,
			URL: "https://example.com/" + date, PublishedAt: date, CollectedAt: date + "T00:00:00Z",
		}).Error; err != nil {
			t.Fatalf("insert article %s: %v", date, err)
		}
	}
	if err := db.Create(&database.DailySummary{Day: "2026-08-01", Summary: "First day", GeneratedAt: "2026-08-01T23:00:00Z"}).Error; err != nil {
		t.Fatalf("insert daily summary: %v", err)
	}

	// When a weekly report covering both days is prepared.
	_, err = NewRepository(db).Build(context.Background(), api.CreateReportRequest{
		Type: "weekly", Start: "2026-08-01", End: "2026-08-02",
	})

	// Then report generation stops instead of silently omitting the second day.
	if err == nil {
		t.Fatal("build succeeded without complete daily summaries")
	}
}
