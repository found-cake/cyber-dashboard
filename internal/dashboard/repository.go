package dashboard

import (
	"context"
	"fmt"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

// noActorLabel mirrors the summary taxonomy sentinel stored for non-incident articles.
const noActorLabel = "None"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Dashboard aggregates the rolling window starting at since (inclusive, YYYY-MM-DD). The
// caller supplies the boundary because it depends on the configured timezone; SQLite's
// date('now') is always UTC and would put the window on a different day. hideNoneActor drops
// the "None" bucket before the top-N cut so the named actors still fill every slot.
func (r *Repository) Dashboard(ctx context.Context, since string, hideNoneActor bool) (api.Dashboard, error) {
	result := api.Dashboard{AttackMethods: []api.BreakdownRow{}, ThreatActors: []api.BreakdownRow{}, CVEs: []api.CVEInsight{}}
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
	// Counted through the articles so the stat row shares one window; the CVE table below it
	// stays unfiltered because the explorer it links to ranks the complete catalogue.
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
	result.CVEs, err = r.cveInsights(ctx)
	if err != nil {
		return api.Dashboard{}, err
	}
	return result, nil
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

func (r *Repository) cveInsights(ctx context.Context) ([]api.CVEInsight, error) {
	insights := []api.CVEInsight{}
	if err := r.db.WithContext(ctx).Table("cves AS c").Select(`c.cve_id AS id, c.cvss_score AS cvss,
		c.affected_product, c.first_seen, COUNT(ac.article_id) AS mentions`).
		Joins("LEFT JOIN article_cves AS ac ON ac.cve_id = c.cve_id").Group("c.cve_id").
		Order("(c.cvss_score + 0.2 * COUNT(ac.article_id)) DESC, c.first_seen DESC, c.cve_id ASC").Scan(&insights).Error; err != nil {
		return nil, fmt.Errorf("query cve insights: %w", err)
	}
	return insights, nil
}
