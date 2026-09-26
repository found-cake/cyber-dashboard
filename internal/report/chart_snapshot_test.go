package report

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestSnapshotsStoreCurrentCountsOnly_whenBaselineExists(t *testing.T) {
	// Given generated reports in two periods with article classifications.
	repository, db := chartRepository(t)
	seedChartArticles(t, db)
	previous, err := repository.Save(context.Background(), api.Report{Type: "weekly", PeriodStart: "2026-07-25", PeriodEnd: "2026-07-31"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	current, err := repository.Save(context.Background(), api.Report{Type: "weekly", PeriodStart: "2026-08-15", PeriodEnd: "2026-08-21"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&database.Article{}).Where("published_at = ?", "2026-07-31").
		Updates(database.Article{AttackMethod: "APT", ThreatActor: "Reclassified actor"}).Error; err != nil {
		t.Fatal(err)
	}
	// When the report is viewed after its baseline article is reclassified.
	got, err := repository.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Then comparison uses the prior report's snapshot and storage contains no copied baseline.
	if got.Charts == nil || got.Charts.Previous == nil || got.Charts.Previous.ReportID != previous.ID ||
		got.Charts.Previous.AttackMethods[0].Label != "Phishing" || got.Charts.Previous.ThreatActors[0].Label != "Old group" {
		t.Fatalf("charts = %+v", got.Charts)
	}
	var stored database.Report
	if err := db.First(&stored, current.ID).Error; err != nil {
		t.Fatal(err)
	}
	var payload struct {
		AttackMethods []api.BreakdownRow `json:"attack_methods"`
		Previous      json.RawMessage    `json:"previous"`
	}
	if err := json.Unmarshal([]byte(stored.Charts), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Previous) != 0 || len(payload.AttackMethods) != 2 {
		t.Fatalf("stored charts = %s", stored.Charts)
	}
}
