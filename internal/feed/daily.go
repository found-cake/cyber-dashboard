package feed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

func (r *Repository) Daily(ctx context.Context, day string) (api.Daily, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, s.name, a.feed_uid, a.title, a.url,
    a.published_at, a.summary, a.attack_method, a.threat_actor, a.actor_country, a.sector, a.severity
    FROM articles a JOIN sources s ON s.id = a.source_id
    WHERE a.published_at = ? ORDER BY a.id DESC`, day)
	if err != nil {
		return api.Daily{}, fmt.Errorf("query daily articles: %w", err)
	}
	daily := api.Daily{Day: day, Articles: []api.Article{}}
	for rows.Next() {
		var article api.Article
		if err := rows.Scan(&article.ID, &article.Source, &article.FeedUID, &article.Title, &article.URL,
			&article.PublishedAt, &article.Summary, &article.AttackMethod, &article.ThreatActor,
			&article.ActorCountry, &article.Sector, &article.Severity); err != nil {
			return api.Daily{}, errors.Join(fmt.Errorf("scan daily article: %w", err), rows.Close())
		}
		daily.Articles = append(daily.Articles, article)
	}
	if err := rows.Err(); err != nil {
		return api.Daily{}, errors.Join(fmt.Errorf("iterate daily articles: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return api.Daily{}, fmt.Errorf("close daily article rows: %w", err)
	}
	for index := range daily.Articles {
		daily.Articles[index].CVEs, err = r.articleCVEs(ctx, daily.Articles[index].ID)
		if err != nil {
			return api.Daily{}, err
		}
	}
	err = r.db.QueryRowContext(ctx, `SELECT summary FROM daily_summaries WHERE day = ?`, day).Scan(&daily.Summary)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return api.Daily{}, fmt.Errorf("query daily summary: %w", err)
	}
	return daily, nil
}

func (r *Repository) SaveDailySummary(ctx context.Context, day, value string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO daily_summaries (day, summary, generated_at)
    VALUES (?, ?, ?) ON CONFLICT(day) DO UPDATE SET summary = excluded.summary,
    generated_at = excluded.generated_at`, day, value, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save daily summary: %w", err)
	}
	return nil
}

func (r *Repository) articleCVEs(ctx context.Context, articleID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT cve_id FROM article_cves WHERE article_id = ? ORDER BY cve_id`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article cves: %w", err)
	}
	defer rows.Close()
	cves := []string{}
	for rows.Next() {
		var cve string
		if err := rows.Scan(&cve); err != nil {
			return nil, fmt.Errorf("scan article cve: %w", err)
		}
		cves = append(cves, cve)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate article cves: %w", err)
	}
	return cves, nil
}

func (r *Repository) CollectedDays(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT published_at FROM articles WHERE published_at != '' ORDER BY published_at`)
	if err != nil {
		return nil, fmt.Errorf("query collected days: %w", err)
	}
	defer rows.Close()
	days := []string{}
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return nil, fmt.Errorf("scan collected day: %w", err)
		}
		days = append(days, day)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collected days: %w", err)
	}
	return days, nil
}
