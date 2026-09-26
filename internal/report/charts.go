package report

import (
	"context"
	"errors"
	"fmt"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

func (r *Repository) withCharts(ctx context.Context, value api.Report) (api.Report, error) {
	switch value.Type {
	case "weekly", "monthly":
	default:
		value.Charts = nil
		return value, nil
	}
	charts := value.Charts
	if charts == nil {
		var err error
		charts, err = r.periodCharts(ctx, valuePeriod{start: value.PeriodStart, end: value.PeriodEnd})
		if err != nil {
			return api.Report{}, err
		}
	}
	if charts == nil {
		return value, nil
	}
	value.Charts = charts
	var previous database.Report
	err := r.db.WithContext(ctx).Where("type = ? AND period_end < ?", value.Type, value.PeriodStart).
		Order("period_end DESC").Order("period_start DESC").Order("id DESC").First(&previous).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, nil
	}
	if err != nil {
		return api.Report{}, fmt.Errorf("query previous report charts: %w", err)
	}
	baseline := decodeChartSnapshot(previous.Charts)
	if baseline == nil {
		baseline, err = r.periodCharts(ctx, valuePeriod{start: previous.PeriodStart, end: previous.PeriodEnd})
		if err != nil {
			return api.Report{}, err
		}
	}
	if baseline != nil {
		charts.Previous = &api.ReportChartBaseline{ReportID: previous.ID, PeriodStart: previous.PeriodStart,
			PeriodEnd: previous.PeriodEnd, AttackMethods: baseline.AttackMethods, ThreatActors: baseline.ThreatActors}
	}
	return value, nil
}

func (r *Repository) periodCharts(ctx context.Context, period valuePeriod) (*api.ReportCharts, error) {
	if period.start == "" || period.end == "" {
		return nil, nil
	}
	methods, err := r.chartBreakdown(ctx, topQuery{column: "attack_method", period: period})
	if err != nil {
		return nil, err
	}
	if len(methods) == 0 {
		return nil, nil
	}
	actors, err := r.chartBreakdown(ctx, topQuery{column: "threat_actor", period: period})
	if err != nil {
		return nil, err
	}
	return &api.ReportCharts{AttackMethods: methods, ThreatActors: actors}, nil
}

func (r *Repository) chartBreakdown(ctx context.Context, query topQuery) ([]api.BreakdownRow, error) {
	switch query.column {
	case "attack_method", "threat_actor":
	default:
		return nil, fmt.Errorf("chart breakdown column %q: invalid", query.column)
	}
	rows := []api.BreakdownRow{}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).
		Select(query.column+" AS label, COUNT(*) AS value").
		Where("published_at BETWEEN ? AND ?", query.period.start, query.period.end).
		Group(query.column).Order("COUNT(*) DESC").Order(query.column + " ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query report %s breakdown: %w", query.column, err)
	}
	return rows, nil
}
