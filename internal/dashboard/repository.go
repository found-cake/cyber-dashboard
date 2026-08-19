package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

// Summary taxonomy sentinels: no incident, then its two unattributed shapes.
const (
	noActorLabel        = "None"
	unknownActorLabel   = "Unknown"
	qualifiedUnknownFmt = "Unknown (%"
)

// DashboardCVELimit is how many CVEs the dashboard card lists; the rest load from GET /api/cves.
const DashboardCVELimit = 8

// CVEPageSize bounds each public CVE explorer response.
const CVEPageSize = 100

// Window is the rolling aggregation window; Bucket divides Days evenly into trend points.
type Window struct {
	// Supplied by the caller because SQLite's date('now') is UTC and would shift the boundary.
	Since  string
	Days   int
	Bucket int
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Dashboard aggregates the window; hideNoneActor drops "None" before the top-N cut.
func (r *Repository) Dashboard(ctx context.Context, window Window, hideNoneActor bool) (api.Dashboard, error) {
	since := window.Since
	result := api.Dashboard{
		AttackMethods: []api.BreakdownRow{}, ThreatActors: []api.BreakdownRow{},
		Trend: []api.TrendPoint{}, CVEs: []api.CVEInsight{},
	}
	var stats struct {
		Total    int
		Critical int
		High     int
	}
	err := r.db.WithContext(ctx).Model(&database.Article{}).
		Select("COUNT(*) AS total", "COALESCE(SUM(severity = 'CRITICAL'), 0) AS critical", "COALESCE(SUM(severity = 'HIGH'), 0) AS high").
		Where("published_at >= ?", since).Scan(&stats).Error
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("query dashboard stats: %w", err)
	}
	result.Total, result.Critical, result.High = stats.Total, stats.Critical, stats.High
	// CVECount follows the selected article window; the card itself is capped below.
	var cveCount int64
	if err := r.db.WithContext(ctx).Table("article_cves AS ac").
		Joins("JOIN articles AS a ON a.id = ac.article_id").
		Where("a.published_at >= ?", since).
		Distinct("ac.cve_id").Count(&cveCount).Error; err != nil {
		return api.Dashboard{}, fmt.Errorf("query cve count: %w", err)
	}
	result.CVECount = int(cveCount)
	result.Empty = result.Total == 0
	result.AttackMethods, err = r.breakdown(ctx, "attack_method", 8, since, false)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.ThreatActors, err = r.breakdown(ctx, "threat_actor", 8, since, hideNoneActor)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.Trend, err = r.trend(ctx, window)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.CVEs, err = r.recentCVEs(ctx, DashboardCVELimit)
	if err != nil {
		return api.Dashboard{}, err
	}
	return result, nil
}

// trend folds per-day rows into buckets in Go; SQLite would have to redo the caller's timezone math.
func (r *Repository) trend(ctx context.Context, window Window) ([]api.TrendPoint, error) {
	if window.Bucket < 1 || window.Days < 1 || window.Days%window.Bucket != 0 {
		return nil, fmt.Errorf("trend window %+v: invalid", window)
	}
	start, err := time.Parse(time.DateOnly, window.Since)
	if err != nil {
		return nil, fmt.Errorf("trend window start %q: %w", window.Since, err)
	}
	var days []struct {
		Day              string
		Total            int
		Critical         int
		High             int
		Medium           int
		Attributed       int
		UnknownActor     int
		QualifiedUnknown int
	}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).
		Select(`published_at AS day, COUNT(*) AS total,
			COALESCE(SUM(severity = 'CRITICAL'), 0) AS critical,
			COALESCE(SUM(severity = 'HIGH'), 0) AS high,
			COALESCE(SUM(severity = 'MEDIUM'), 0) AS medium,
			COALESCE(SUM(threat_actor <> ? AND threat_actor <> ''), 0) AS attributed,
			COALESCE(SUM(threat_actor = ?), 0) AS unknown_actor,
			COALESCE(SUM(threat_actor LIKE ?), 0) AS qualified_unknown`,
			noActorLabel, unknownActorLabel, qualifiedUnknownFmt).
		Where("published_at >= ?", window.Since).Group("published_at").Scan(&days).Error; err != nil {
		return nil, fmt.Errorf("query trend: %w", err)
	}

	count := window.Days / window.Bucket
	points := make([]api.TrendPoint, count)
	for index := range points {
		bucketStart := start.AddDate(0, 0, index*window.Bucket)
		points[index] = api.TrendPoint{
			Start: bucketStart.Format(time.DateOnly),
			End:   bucketStart.AddDate(0, 0, window.Bucket-1).Format(time.DateOnly),
		}
	}
	for _, day := range days {
		parsed, parseErr := time.Parse(time.DateOnly, day.Day)
		if parseErr != nil {
			continue
		}
		index := int(parsed.Sub(start).Hours()/24) / window.Bucket
		if index < 0 || index >= count {
			continue
		}
		point := &points[index]
		point.Total += day.Total
		point.Critical += day.Critical
		point.High += day.High
		point.Medium += day.Medium
		point.Attributed += day.Attributed
		point.UnknownActor += day.UnknownActor
		point.QualifiedUnknown += day.QualifiedUnknown
	}
	for index := range points {
		point := &points[index]
		point.NamedActor = point.Attributed - point.UnknownActor - point.QualifiedUnknown
	}
	return points, nil
}

func (r *Repository) breakdown(ctx context.Context, column string, limit int, since string, excludeNone bool) ([]api.BreakdownRow, error) {
	var selected string
	switch column {
	case "attack_method":
		selected = "attack_method"
	case "threat_actor":
		selected = "threat_actor"
	default:
		return nil, fmt.Errorf("breakdown column %q: invalid", column)
	}
	result := []api.BreakdownRow{}
	query := r.db.WithContext(ctx).Model(&database.Article{}).Select(selected+" AS label, COUNT(*) AS value").
		Where("published_at >= ?", since)
	if excludeNone {
		query = query.Where(selected+" <> ?", noActorLabel)
	}
	if err := query.Group(selected).Order("COUNT(*) DESC").Limit(limit).Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("query %s breakdown: %w", column, err)
	}
	return result, nil
}
