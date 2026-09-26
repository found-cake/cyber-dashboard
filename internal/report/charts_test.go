package report

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

func chartRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "charts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(db); err != nil {
			t.Error(err)
		}
	})
	return NewRepository(db), db
}

func seedChartArticles(t *testing.T, db *gorm.DB) {
	t.Helper()
	for index := range 10 {
		article := database.Article{SourceID: 1, FeedUID: fmt.Sprintf("chart-%d", index),
			Title: fmt.Sprintf("Incident %d", index), URL: fmt.Sprintf("https://example.com/%d", index),
			PublishedAt: "2026-08-15", CollectedAt: "2026-08-15T00:00:00Z",
			AttackMethod: "Ransomware", ThreatActor: fmt.Sprintf("Group %d", index)}
		if index == 0 {
			article.AttackMethod, article.ThreatActor = "Unclassified", "Unknown"
		}
		if index == 1 {
			article.ThreatActor = "None"
		}
		if err := db.Create(&article).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&database.DailySummary{Day: "2026-08-15", Summary: "Daily digest", GeneratedAt: "2026-08-16"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.Article{SourceID: 1, FeedUID: "outside-period", Title: "Old incident",
		URL: "https://example.com/old", PublishedAt: "2026-07-31", CollectedAt: "2026-07-31",
		AttackMethod: "Phishing", ThreatActor: "Old group"}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestSaveCharts_whenReportIsPeriodic(t *testing.T) {
	// Given classified articles with more actors than the dashboard's display limit.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	for _, kind := range []string{"weekly", "monthly", "daily"} {
		t.Run(kind, func(t *testing.T) {
			// When a report is built for the article date.
			report, err := repository.Save(context.Background(), api.Report{Type: kind, PeriodStart: "2026-08-15", PeriodEnd: "2026-08-15"}, 0)
			if err != nil {
				t.Fatal(err)
			}
			// Then only periodic reports carry full, dashboard-compatible chart counts.
			if kind == "daily" {
				if report.Charts != nil {
					t.Fatal("daily report has charts")
				}
				return
			}
			charts := report.Charts
			if charts == nil || len(charts.ThreatActors) != 10 || charts.Previous != nil {
				t.Fatalf("charts = %+v", charts)
			}
			want := []api.BreakdownRow{{Label: "Ransomware", Value: 9}, {Label: "Unclassified", Value: 1}}
			if !reflect.DeepEqual(charts.AttackMethods, want) {
				t.Fatalf("attack methods = %+v, want %+v", charts.AttackMethods, want)
			}
		})
	}
}

func TestGetChartsSelectsLatestEarlierPeriod_whenLegacyReportsCreatedOutOfOrder(t *testing.T) {
	// Given earlier reports inserted out of order plus overlapping and other-type reports.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	for _, item := range []database.Report{
		{ID: 10, Type: "weekly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-07", GeneratedAt: "2026-09-01"},
		{ID: 11, Type: "weekly", PeriodStart: "2026-08-08", PeriodEnd: "2026-08-14", GeneratedAt: "2026-08-15"},
		{ID: 12, Type: "weekly", PeriodStart: "2026-08-15", PeriodEnd: "2026-08-21", GeneratedAt: "2026-09-03"},
		{ID: 13, Type: "monthly", PeriodStart: "2026-08-01", PeriodEnd: "2026-08-14", GeneratedAt: "2026-09-04"},
		{ID: 14, Type: "weekly", PeriodStart: "2026-08-14", PeriodEnd: "2026-08-16", GeneratedAt: "2026-09-05"},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&database.Article{SourceID: 1, FeedUID: "previous", Title: "Previous incident", URL: "https://example.com/previous",
		PublishedAt: "2026-08-10", CollectedAt: "2026-08-10", AttackMethod: "APT", ThreatActor: "Group A"}).Error; err != nil {
		t.Fatal(err)
	}
	// When the report period starts after the two completed weekly periods.
	report, err := repository.Get(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	// Then period chronology selects the immediate prior report and its stored counts.
	charts := report.Charts
	if charts == nil || charts.Previous == nil || charts.Previous.ReportID != 11 || charts.Previous.PeriodEnd != "2026-08-14" {
		t.Fatalf("charts = %+v", charts)
	}
	if charts.Previous.AttackMethods[0].Value != 1 || charts.Previous.ThreatActors[0].Label != "Group A" {
		t.Fatalf("baseline = %+v", charts.Previous)
	}
}

func TestSaveChartsOmitsBaseline_whenLatestPreviousReportHasNoArticles(t *testing.T) {
	// Given an earlier report with articles and the latest previous report without articles.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	if err := db.Create(&database.Report{Type: "weekly", PeriodStart: "2026-07-25", PeriodEnd: "2026-07-31"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.Report{Type: "weekly", PeriodStart: "2026-08-08", PeriodEnd: "2026-08-14"}).Error; err != nil {
		t.Fatal(err)
	}
	// When the next periodic report is built.
	report, err := repository.Save(context.Background(), api.Report{Type: "weekly", PeriodStart: "2026-08-15", PeriodEnd: "2026-08-21"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Then missing historical counts are not fabricated as zero.
	if report.Charts == nil || report.Charts.Previous != nil {
		t.Fatalf("charts = %+v", report.Charts)
	}
}

func TestGetPreservesCurrentCounts_whenArticlesChangeAndBaselineIsDeleted(t *testing.T) {
	// Given saved reports whose graphs are derived from article dates.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	input := api.Report{Type: "weekly", PeriodStart: "2026-08-15", PeriodEnd: "2026-08-21"}
	if err := db.Create(&database.Report{ID: 100, Type: "weekly", PeriodStart: "2026-07-25", PeriodEnd: "2026-07-31"}).Error; err != nil {
		t.Fatal(err)
	}
	saved, err := repository.Save(context.Background(), input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Charts == nil || saved.Charts.Previous == nil {
		t.Fatal("saved report missing live chart baseline")
	}
	if err := db.Model(&database.Article{}).Where("published_at = ?", "2026-08-15").Update("attack_method", "Phishing").Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	// When the persisted report is retrieved after live data changes.
	got, err := repository.Get(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Then generated counts remain fixed while deleted baseline availability changes immediately.
	if got.Charts == nil || got.Charts.Previous != nil || !reflect.DeepEqual(got.Charts.AttackMethods, saved.Charts.AttackMethods) {
		t.Fatalf("charts = %+v", got.Charts)
	}
}
