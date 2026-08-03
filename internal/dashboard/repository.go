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

func (r *Repository) Dashboard(ctx context.Context) (api.Dashboard, error) {
	result := api.Dashboard{AttackMethods: []api.BreakdownRow{}, ThreatActors: []api.BreakdownRow{}, CVEs: []api.CVEInsight{}}
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),
    COALESCE(SUM(severity = 'CRITICAL'), 0), COALESCE(SUM(severity = 'HIGH'), 0)
    FROM articles WHERE published_at >= date('now', '-29 days')`).Scan(&result.Total, &result.Critical, &result.High)
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("query dashboard stats: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cves`).Scan(&result.CVECount); err != nil {
		return api.Dashboard{}, fmt.Errorf("query cve count: %w", err)
	}
	result.Empty = result.Total == 0
	result.AttackMethods, err = r.breakdown(ctx, "attack_method", 8)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.ThreatActors, err = r.breakdown(ctx, "threat_actor", 7)
	if err != nil {
		return api.Dashboard{}, err
	}
	result.CVEs, err = r.cveInsights(ctx)
	if err != nil {
		return api.Dashboard{}, err
	}
	return result, nil
}

func (r *Repository) breakdown(ctx context.Context, column string, limit int) ([]api.BreakdownRow, error) {
	if column != "attack_method" && column != "threat_actor" {
		return nil, fmt.Errorf("breakdown column %q: invalid", column)
	}
	query := fmt.Sprintf(`SELECT %s, COUNT(*) FROM articles
    WHERE published_at >= date('now', '-29 days') GROUP BY %s ORDER BY COUNT(*) DESC LIMIT ?`, column, column)
	rows, err := r.db.QueryContext(ctx, query, limit)
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
