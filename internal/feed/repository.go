package feed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/severity"
)

var ErrNotFound = errors.New("feed item not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Sources(ctx context.Context) ([]api.Source, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, host, slug, enabled FROM sources ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()
	sources := []api.Source{}
	for rows.Next() {
		var source api.Source
		if err := rows.Scan(&source.ID, &source.Name, &source.Host, &source.Slug, &source.Enabled); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources: %w", err)
	}
	return sources, nil
}

func (r *Repository) SetSourceEnabled(ctx context.Context, id int64, enabled bool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE sources SET enabled = ? WHERE id = ?`, enabled, id)
	if err != nil {
		return fmt.Errorf("update source %d: %w", id, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("source rows affected: %w", err)
	}
	if changed == 0 {
		return fmt.Errorf("source %d: %w", id, ErrNotFound)
	}
	return nil
}

func (r *Repository) SaveArticle(ctx context.Context, source api.Source, article FeedArticle, day string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin article transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	description := cleanText(article.Description)
	body := strings.TrimSpace(article.Body)
	method := "Unclassified"
	if len(article.Categories) > 0 && strings.TrimSpace(article.Categories[0]) != "" {
		method = strings.TrimSpace(article.Categories[0])
	}
	initialSeverity := severity.Unknown
	if source.Slug == "stepsecurity" {
		initialSeverity = severity.High
	}
	var articleID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO articles
	(source_id, feed_uid, title, url, published_at, published_time, collected_at, body, summary, attack_method, threat_actor, severity)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(feed_uid) DO UPDATE SET title = excluded.title, url = excluded.url,
	published_at = excluded.published_at, published_time = excluded.published_time,
	body = excluded.body, summary = excluded.summary, attack_method = excluded.attack_method,
	severity = CASE WHEN excluded.severity = 'HIGH' AND articles.severity IN ('UNKNOWN', 'LOW', 'MEDIUM')
		THEN excluded.severity ELSE articles.severity END
	RETURNING id`,
		source.ID, article.ID, cleanText(article.Title), article.URL, day, publishedTimestamp(article, day),
		time.Now().UTC().Format(time.RFC3339), body, description, method, "Unknown", string(initialSeverity)).Scan(&articleID)
	if err != nil {
		return fmt.Errorf("upsert article %s: %w", article.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM article_cves WHERE article_id = ?`, articleID); err != nil {
		return fmt.Errorf("clear article %d CVE links: %w", articleID, err)
	}
	for _, cve := range extractCVEs(article.Title + " " + description + " " + body) {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO cves (cve_id, first_seen)
			SELECT ?, ? WHERE NOT EXISTS (SELECT 1 FROM rejected_cves WHERE cve_id = ?)`, cve, day, cve); err != nil {
			return fmt.Errorf("insert cve %s: %w", cve, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO article_cves (article_id, cve_id)
			SELECT ?, ? WHERE EXISTS (SELECT 1 FROM cves WHERE cve_id = ?)`, articleID, cve, cve); err != nil {
			return fmt.Errorf("link cve %s: %w", cve, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit article: %w", err)
	}
	return nil
}
