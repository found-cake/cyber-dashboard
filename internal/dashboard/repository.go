package dashboard

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/found-cake/cyber-dashboard/api"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Dashboard aggregates the rolling window starting at since (inclusive, YYYY-MM-DD). The
// caller supplies the boundary because it depends on the configured timezone; SQLite's
// date('now') is always UTC and would put the window on a different day.
func (r *Repository) Dashboard(ctx context.Context, since string) (api.Dashboard, error) {
	result := api.Dashboard{AttackMethods: []api.BreakdownRow{}, ThreatActors: []api.BreakdownRow{}, CVEs: []api.CVEInsight{}}
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),
    COALESCE(SUM(severity = 'CRITICAL'), 0), COALESCE(SUM(severity = 'HIGH'), 0)
    FROM articles WHERE published_at >= ?`, since).Scan(&result.Total, &result.Critical, &result.High)
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("query dashboard stats: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cves`).Scan(&result.CVECount); err != nil {
		return api.Dashboard{}, fmt.Errorf("query cve count: %w", err)
	}
	result.Empty = result.Total == 0
	result.AttackMethods, err = r.breakdown(ctx, "attack_method", 8, since)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.ThreatActors, err = r.breakdown(ctx, "threat_actor", 7, since)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.CVEs, err = r.cveInsights(ctx)
	if err != nil {
		return api.Dashboard{}, err
	}
	return result, nil
}

func (r *Repository) breakdown(ctx context.Context, column string, limit int, since string) ([]api.BreakdownRow, error) {
	var query string
	switch column {
	case "attack_method":
		query = `SELECT attack_method, COUNT(*) FROM articles
    WHERE published_at >= ? GROUP BY attack_method ORDER BY COUNT(*) DESC LIMIT ?`
	case "threat_actor":
		query = `SELECT threat_actor, COUNT(*) FROM articles
    WHERE published_at >= ? GROUP BY threat_actor ORDER BY COUNT(*) DESC LIMIT ?`
	default:
		return nil, fmt.Errorf("breakdown column %q: invalid", column)
	}
	rows, err := r.db.QueryContext(ctx, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("query %s breakdown: %w", column, err)
	}
	defer rows.Close()
	result := []api.BreakdownRow{}
	for rows.Next() {
		var row api.BreakdownRow
		if err := rows.Scan(&row.Label, &row.Value); err != nil {
			return nil, fmt.Errorf("scan %s breakdown: %w", column, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s breakdown: %w", column, err)
	}
	return result, nil
}

func (r *Repository) cveInsights(ctx context.Context) ([]api.CVEInsight, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.cve_id, c.cvss_score, c.affected_product, c.first_seen, COUNT(ac.article_id)
    FROM cves c LEFT JOIN article_cves ac ON ac.cve_id = c.cve_id
    GROUP BY c.cve_id
    ORDER BY (c.cvss_score + 0.2 * COUNT(ac.article_id)) DESC, c.first_seen DESC, c.cve_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query cve insights: %w", err)
	}
	defer rows.Close()
	insights := []api.CVEInsight{}
	for rows.Next() {
		var insight api.CVEInsight
		if err := rows.Scan(&insight.ID, &insight.CVSS, &insight.AffectedProduct, &insight.FirstSeen, &insight.Mentions); err != nil {
			return nil, fmt.Errorf("scan cve insight: %w", err)
		}
		insights = append(insights, insight)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cve insights: %w", err)
	}
	return insights, nil
}
