package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("report data not found")
var ErrDailySummariesRequired = errors.New("daily summaries are required for the report period")

type Repository struct {
	db  *gorm.DB
	now func() time.Time
}

type draft struct {
	report  api.Report
	facts   []string
	threats []threatCandidate
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, now: time.Now}
}

func (r *Repository) List(ctx context.Context) ([]api.Report, error) {
	var stored []database.Report
	if err := r.db.WithContext(ctx).Order("generated_at DESC").Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	reports := make([]api.Report, 0, len(stored))
	for _, item := range stored {
		threats := decodeValues[api.ReportThreat](item.TopThreats)
		if len(threats) == 0 && item.TopThreat != "" {
			threats = []api.ReportThreat{{Title: item.TopThreat, SourceCount: 1}}
		}
		reports = append(reports, api.Report{ID: item.ID, Type: item.Type, PeriodStart: item.PeriodStart,
			PeriodEnd: item.PeriodEnd, Total: item.Total, Critical: item.Critical, High: item.High,
			Medium: item.Medium, TopThreat: item.TopThreat, TopThreats: threats, Actors: decodeValues[string](item.Actors),
			Sectors: decodeValues[string](item.Sectors), Summary: item.Summary, GeneratedAt: item.GeneratedAt})
	}
	return reports, nil
}

func (r *Repository) Build(ctx context.Context, request api.CreateReportRequest) (draft, error) {
	value := api.Report{Type: request.Type, PeriodStart: request.Start, PeriodEnd: request.End, Actors: []string{}, Sectors: []string{}}
	var counts struct {
		Total    int
		Critical int
		High     int
		Medium   int
	}
	err := r.db.WithContext(ctx).Model(&database.Article{}).
		Select("COUNT(*) AS total", "COALESCE(SUM(severity = 'CRITICAL'), 0) AS critical",
			"COALESCE(SUM(severity = 'HIGH'), 0) AS high", "COALESCE(SUM(severity = 'MEDIUM'), 0) AS medium").
		Where("published_at BETWEEN ? AND ?", request.Start, request.End).Scan(&counts).Error
	if err != nil {
		return draft{}, fmt.Errorf("aggregate report: %w", err)
	}
	value.Total, value.Critical, value.High, value.Medium = counts.Total, counts.Critical, counts.High, counts.Medium
	if value.Total == 0 {
		return draft{}, fmt.Errorf("report %s to %s: %w", request.Start, request.End, ErrNotFound)
	}
	period := valuePeriod{start: request.Start, end: request.End}
	value.Actors, err = r.topValues(ctx, topQuery{column: "threat_actor", period: period, limit: 6})
	if err != nil {
		return draft{}, err
	}
	value.Sectors, err = r.topValues(ctx, topQuery{column: "sector", period: period, limit: 5})
	if err != nil {
		return draft{}, err
	}
	var facts []string
	switch request.Type {
	case "weekly", "monthly":
		facts, err = r.dailySummaryFacts(ctx, period)
	default:
		facts, err = r.articleFacts(ctx, period)
	}
	if err != nil {
		return draft{}, err
	}
	limit := topThreatLimit(request.Type)
	threats, err := r.threatCandidates(ctx, period, limit*2)
	if err != nil {
		return draft{}, err
	}
	value.TopThreats = staticReportThreats(threats, limit)
	value.TopThreat = firstThreatTitle(value.TopThreats)
	return draft{report: value, facts: facts, threats: threats}, nil
}

func (r *Repository) Save(ctx context.Context, value api.Report, timezoneOffsetMinutes int) (api.Report, error) {
	location := time.FixedZone("configured", timezoneOffsetMinutes*60)
	value.GeneratedAt = r.now().In(location).Format(time.RFC3339)
	actors, err := encodeValues(value.Actors)
	if err != nil {
		return api.Report{}, err
	}
	sectors, err := encodeValues(value.Sectors)
	if err != nil {
		return api.Report{}, err
	}
	threats, err := encodeValues(value.TopThreats)
	if err != nil {
		return api.Report{}, err
	}
	stored := database.Report{Type: value.Type, PeriodStart: value.PeriodStart, PeriodEnd: value.PeriodEnd,
		Total: value.Total, Critical: value.Critical, High: value.High, Medium: value.Medium,
		TopThreat: value.TopThreat, TopThreats: threats, Actors: actors, Sectors: sectors, Summary: value.Summary, GeneratedAt: value.GeneratedAt}
	if err := r.db.WithContext(ctx).Create(&stored).Error; err != nil {
		return api.Report{}, fmt.Errorf("insert report: %w", err)
	}
	value.ID = stored.ID
	return value, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&database.Report{})
	if result.Error != nil {
		return fmt.Errorf("delete report %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("report %d: %w", id, ErrNotFound)
	}
	return nil
}

func encodeValues[T any](values []T) (string, error) {
	if values == nil {
		values = []T{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode report list: %w", err)
	}
	return string(encoded), nil
}

func decodeValues[T any](value string) []T {
	var values []T
	if err := json.Unmarshal([]byte(value), &values); err != nil || values == nil {
		return []T{}
	}
	return values
}
