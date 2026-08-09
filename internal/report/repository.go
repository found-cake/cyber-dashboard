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

type Repository struct {
	db  *gorm.DB
	now func() time.Time
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
		reports = append(reports, api.Report{ID: item.ID, Type: item.Type, PeriodStart: item.PeriodStart,
			PeriodEnd: item.PeriodEnd, Total: item.Total, Critical: item.Critical, High: item.High,
			Medium: item.Medium, TopThreat: item.TopThreat, Actors: decodeList(item.Actors),
			Sectors: decodeList(item.Sectors), Summary: item.Summary, GeneratedAt: item.GeneratedAt})
	}
	return reports, nil
}

func (r *Repository) Build(ctx context.Context, request api.CreateReportRequest) (api.Report, []string, error) {
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
		return api.Report{}, nil, fmt.Errorf("aggregate report: %w", err)
	}
	value.Total, value.Critical, value.High, value.Medium = counts.Total, counts.Critical, counts.High, counts.Medium
	if value.Total == 0 {
		return api.Report{}, nil, fmt.Errorf("report %s to %s: %w", request.Start, request.End, ErrNotFound)
	}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).Select("title").Where("published_at BETWEEN ? AND ?", request.Start, request.End).
		Order("CASE severity WHEN 'CRITICAL' THEN 0 WHEN 'HIGH' THEN 1 ELSE 2 END, published_at DESC").Limit(1).Scan(&value.TopThreat).Error; err != nil {
		return api.Report{}, nil, fmt.Errorf("query top threat: %w", err)
	}
	period := valuePeriod{start: request.Start, end: request.End}
	value.Actors, err = r.topValues(ctx, topQuery{column: "threat_actor", period: period, limit: 6})
	if err != nil {
		return api.Report{}, nil, err
	}
	value.Sectors, err = r.topValues(ctx, topQuery{column: "sector", period: period, limit: 5})
	if err != nil {
		return api.Report{}, nil, err
	}
	facts, err := r.articleFacts(ctx, period)
	if err != nil {
		return api.Report{}, nil, err
	}
	return value, facts, nil
}

func (r *Repository) Save(ctx context.Context, value api.Report, timezoneOffsetMinutes int) (api.Report, error) {
	location := time.FixedZone("configured", timezoneOffsetMinutes*60)
	value.GeneratedAt = r.now().In(location).Format(time.RFC3339)
	actors, err := encodeList(value.Actors)
	if err != nil {
		return api.Report{}, err
	}
	sectors, err := encodeList(value.Sectors)
	if err != nil {
		return api.Report{}, err
	}
	stored := database.Report{Type: value.Type, PeriodStart: value.PeriodStart, PeriodEnd: value.PeriodEnd,
		Total: value.Total, Critical: value.Critical, High: value.High, Medium: value.Medium,
		TopThreat: value.TopThreat, Actors: actors, Sectors: sectors, Summary: value.Summary, GeneratedAt: value.GeneratedAt}
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

// encodeList stores a string list as a JSON array. Actor and sector names come from LLM
// analysis of article text and routinely contain commas ("Scattered Spider, Inc.",
// "금융, 보험"), which a comma-joined column cannot round-trip.
func encodeList(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode report list: %w", err)
	}
	return string(encoded), nil
}

func decodeList(value string) []string {
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil || values == nil {
		return []string{}
	}
	return values
}
