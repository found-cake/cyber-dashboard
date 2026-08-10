package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/severity"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"gorm.io/gorm"
)

type ArticleForAnalysis struct {
	ID    int64
	Title string
	URL   string
	Body  string
}

func (r *Repository) ArticlesForAnalysis(ctx context.Context, day string) ([]ArticleForAnalysis, error) {
	articles := []ArticleForAnalysis{}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).Select("id", "title", "url", "body").
		Where("published_at = ? AND body != ''", day).Order("published_time, id").Find(&articles).Error; err != nil {
		return nil, fmt.Errorf("query articles for analysis: %w", err)
	}
	return articles, nil
}

func (r *Repository) SaveArticleAnalysis(ctx context.Context, articleID int64, analysis summary.ArticleAnalysis) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&database.Article{}).Where("id = ?", articleID).Updates(map[string]any{
			"summary": analysis.Summary, "attack_method": analysis.AttackMethod, "threat_actor": analysis.ThreatActor,
			"actor_country": analysis.ActorCountry, "sector": analysis.TargetSector, "victim_count": analysis.VictimCount,
			"damage_usd": analysis.DamageUSD, "zero_day": analysis.ZeroDay, "patch_available": analysis.PatchAvailable,
		})
		if result.Error != nil {
			return fmt.Errorf("update article analysis: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("article %d: %w", articleID, ErrNotFound)
		}
		return recalculateArticleSeverity(ctx, tx, articleID)
	})
}

func recalculateArticleSeverity(ctx context.Context, tx *gorm.DB, articleID int64) error {
	var values struct {
		Score          float64
		Vector         string
		VictimCount    int
		DamageUSD      int64
		ZeroDay        bool
		PatchAvailable string
		AttackMethod   string
		Sector         string
		SourceSlug     string
	}
	// The vector comes from the CVE holding the highest score, so the adjustment is applied
	// to the same flaw the score was taken from rather than to whichever CVE sorts first.
	err := tx.WithContext(ctx).Raw(`SELECT COALESCE(MAX(c.cvss_score), 0) AS score,
		COALESCE((SELECT c2.cvss_vector FROM article_cves ac2 JOIN cves c2 ON c2.cve_id = ac2.cve_id
			WHERE ac2.article_id = a.id ORDER BY c2.cvss_score DESC LIMIT 1), '') AS vector,
		a.victim_count, a.damage_usd, a.zero_day, a.patch_available, a.attack_method, a.sector, s.slug AS source_slug
		FROM articles a LEFT JOIN article_cves ac ON ac.article_id = a.id
		LEFT JOIN cves c ON c.cve_id = ac.cve_id JOIN sources s ON s.id = a.source_id
		WHERE a.id = ? GROUP BY a.id, s.slug`, articleID).Scan(&values).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && values.SourceSlug == "") {
		return fmt.Errorf("article %d: %w", articleID, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("calculate article severity: %w", err)
	}
	sourceFloor := severity.Unknown
	if values.SourceSlug == "stepsecurity" {
		sourceFloor = severity.High
	}
	level := severity.Max(sourceFloor, severity.FromVulnerability(values.Score, values.Vector, values.PatchAvailable),
		severity.FromContext(values.VictimCount, values.ZeroDay), severity.FromDamage(values.DamageUSD))
	// What the article reports decides how its measurements are read. An advisory borrows
	// its whole severity from a CVSS score for a flaw nobody has used yet, which is how a
	// patch notice ends up ranked beside an encrypted hospital; a real intrusion that named
	// no number is not therefore harmless. So a non-incident is held below the top band, and
	// an incident keeps a floor even when the article counts nothing.
	if summary.IsIncidentMethod(values.AttackMethod) {
		level = severity.Max(level, severity.Medium)
		if summary.IsHighImpactSector(values.Sector) {
			level = severity.Raise(level)
		}
	} else {
		level = severity.Cap(level, severity.High)
	}
	if err := tx.Model(&database.Article{}).Where("id = ?", articleID).Update("severity", string(level)).Error; err != nil {
		return fmt.Errorf("update article severity: %w", err)
	}
	return nil
}
