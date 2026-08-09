package feed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/severity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("feed item not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Sources(ctx context.Context) ([]api.Source, error) {
	var stored []database.Source
	if err := r.db.WithContext(ctx).Order("id").Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	sources := make([]api.Source, 0, len(stored))
	for _, source := range stored {
		sources = append(sources, api.Source{ID: source.ID, Name: source.Name, Host: source.Host, Slug: source.Slug, Enabled: source.Enabled})
	}
	return sources, nil
}

func (r *Repository) SetSourceEnabled(ctx context.Context, id int64, enabled bool) error {
	result := r.db.WithContext(ctx).Model(&database.Source{}).Where("id = ?", id).Update("enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("update source %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("source %d: %w", id, ErrNotFound)
	}
	return nil
}

func (r *Repository) SaveArticle(ctx context.Context, source api.Source, article FeedArticle, day string) error {
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
	stored := database.Article{SourceID: source.ID, FeedUID: article.ID, Title: cleanText(article.Title), URL: article.URL,
		PublishedAt: day, PublishedTime: publishedTimestamp(article, day), CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Body: body, Summary: description, AttackMethod: method, ThreatActor: "Unknown", Severity: string(initialSeverity)}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"title": stored.Title, "url": stored.URL, "published_at": stored.PublishedAt,
			"published_time": stored.PublishedTime, "body": stored.Body, "summary": stored.Summary,
			"attack_method": stored.AttackMethod,
			"severity":      gorm.Expr("CASE WHEN excluded.severity = 'HIGH' AND articles.severity IN ('UNKNOWN', 'LOW', 'MEDIUM') THEN excluded.severity ELSE articles.severity END"),
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "feed_uid"}}, DoUpdates: clause.Assignments(updates)}, clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(&stored).Error; err != nil {
			return fmt.Errorf("upsert article %s: %w", article.ID, err)
		}
		if err := tx.Where("article_id = ?", stored.ID).Delete(&database.ArticleCVE{}).Error; err != nil {
			return fmt.Errorf("clear article %d CVE links: %w", stored.ID, err)
		}
		for _, cveID := range extractCVEs(article.Title + " " + description + " " + body) {
			var rejected int64
			if err := tx.Model(&database.RejectedCVE{}).Where("cve_id = ?", cveID).Count(&rejected).Error; err != nil {
				return fmt.Errorf("check rejected CVE %s: %w", cveID, err)
			}
			if rejected > 0 {
				continue
			}
			cve := database.CVE{CVEID: cveID, FirstSeen: day}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&cve).Error; err != nil {
				return fmt.Errorf("insert cve %s: %w", cveID, err)
			}
			link := database.ArticleCVE{ArticleID: stored.ID, CVEID: cveID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
				return fmt.Errorf("link cve %s: %w", cveID, err)
			}
		}
		return nil
	})
}
