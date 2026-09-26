package report

import (
	"context"
	"reflect"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestGetOmitsCharts_whenCurrentPeriodHasNoArticlesOrIsDaily(t *testing.T) {
	// Given legacy reports with missing dates, unavailable articles, or daily type.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	for _, item := range []database.Report{
		{ID: 1, Type: "weekly", PeriodStart: "2026-08-08", PeriodEnd: "2026-08-14"},
		{ID: 2, Type: "weekly", PeriodStart: "", PeriodEnd: "2026-08-21"},
		{ID: 3, Type: "monthly", PeriodStart: "2026-08-01", PeriodEnd: ""},
		{ID: 4, Type: "daily", PeriodStart: "2026-08-15", PeriodEnd: "2026-08-15"},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		// When each stored report is read.
		got, err := repository.Get(context.Background(), item.ID)
		if err != nil {
			t.Fatal(err)
		}
		// Then an absent dataset is never represented as a zero-count comparison.
		if got.Charts != nil {
			t.Fatalf("report %d charts = %+v", item.ID, got.Charts)
		}
	}
}

func TestListRecalculatesBothPeriods_whenArticlesAreReclassified(t *testing.T) {
	// Given legacy report records and newly reclassified articles in both periods.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	for _, item := range []database.Report{
		{ID: 1, Type: "weekly", PeriodStart: "2026-07-25", PeriodEnd: "2026-07-31", GeneratedAt: "2026-08-01"},
		{ID: 2, Type: "weekly", PeriodStart: "2026-08-15", PeriodEnd: "2026-08-21", GeneratedAt: "2026-08-22"},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&database.Article{}).Where("published_at IN ?", []string{"2026-07-31", "2026-08-15"}).
		Updates(database.Article{AttackMethod: "APT", ThreatActor: "Updated actor"}).Error; err != nil {
		t.Fatal(err)
	}
	// When full report responses are listed.
	reports, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Then both current and comparison counts reflect the current article data.
	if len(reports) != 2 || reports[0].Charts == nil || reports[0].Charts.Previous == nil {
		t.Fatalf("reports = %+v", reports)
	}
	charts := reports[0].Charts
	if !reflect.DeepEqual(charts.AttackMethods, []api.BreakdownRow{{Label: "APT", Value: 10}}) ||
		!reflect.DeepEqual(charts.Previous.ThreatActors, []api.BreakdownRow{{Label: "Updated actor", Value: 1}}) {
		t.Fatalf("charts = %+v", charts)
	}
}
