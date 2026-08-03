package feed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/found-cake/cyber-dashboard/internal/severity"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

type ArticleForAnalysis struct {
	ID    int64
	Title string
	URL   string
	Body  string
}

func (r *Repository) ArticlesForAnalysis(ctx context.Context, day string) ([]ArticleForAnalysis, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, title, url, body FROM articles
		WHERE published_at = ? AND body != '' ORDER BY published_time, id`, day)
	if err != nil {
		return nil, fmt.Errorf("query articles for analysis: %w", err)
	}
	defer rows.Close()
	articles := []ArticleForAnalysis{}
	for rows.Next() {
		var article ArticleForAnalysis
		if err := rows.Scan(&article.ID, &article.Title, &article.URL, &article.Body); err != nil {
			return nil, fmt.Errorf("scan article for analysis: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate articles for analysis: %w", err)
	}
	return articles, nil
}

func (r *Repository) SaveArticleAnalysis(ctx context.Context, articleID int64, analysis summary.ArticleAnalysis) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin article analysis transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE articles SET summary = ?, attack_method = ?, threat_actor = ?,
		actor_country = ?, sector = ?, victim_count = ?, zero_day = ? WHERE id = ?`,
		analysis.Summary, analysis.AttackMethod, analysis.ThreatActor, analysis.ActorCountry,
		analysis.TargetSector, analysis.VictimCount, analysis.ZeroDay, articleID)
	if err != nil {
		return fmt.Errorf("update article analysis: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("article analysis rows affected: %w", err)
	}
	if changed == 0 {
		return fmt.Errorf("article %d: %w", articleID, ErrNotFound)
	}
	if err := recalculateArticleSeverity(ctx, tx, articleID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit article analysis: %w", err)
	}
	return nil
}

func recalculateArticleSeverity(ctx context.Context, tx *sql.Tx, articleID int64) error {
	var score float64
	var victimCount int
	var zeroDay bool
	var sourceSlug string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(c.cvss_score), 0), a.victim_count, a.zero_day, s.slug
		FROM articles a LEFT JOIN article_cves ac ON ac.article_id = a.id
		LEFT JOIN cves c ON c.cve_id = ac.cve_id JOIN sources s ON s.id = a.source_id
		WHERE a.id = ? GROUP BY a.id, s.slug`, articleID).
		Scan(&score, &victimCount, &zeroDay, &sourceSlug)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("article %d: %w", articleID, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("calculate article severity: %w", err)
	}
	sourceFloor := severity.Unknown
	if sourceSlug == "stepsecurity" {
		sourceFloor = severity.High
	}
	level := severity.Max(sourceFloor, severity.FromCVSS(score), severity.FromContext(victimCount, zeroDay))
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET severity = ? WHERE id = ?`, string(level), articleID); err != nil {
		return fmt.Errorf("update article severity: %w", err)
	}
	return nil
}
