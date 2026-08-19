package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ensureCVERankingSchema(ctx context.Context, db *gorm.DB, backfill bool) error {
	if backfill {
		if err := db.WithContext(ctx).Exec(`UPDATE cves SET mention_count =
      (SELECT COUNT(*) FROM article_cves WHERE article_cves.cve_id = cves.cve_id)`).Error; err != nil {
			return fmt.Errorf("backfill CVE mention counts: %w", err)
		}
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&cveState{ID: 1, Revision: 1}).Error; err != nil {
		return fmt.Errorf("initialize CVE revision: %w", err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE cve_states SET cve_count = (SELECT COUNT(*) FROM cves) WHERE id = 1`).Error; err != nil {
		return fmt.Errorf("initialize CVE count: %w", err)
	}
	statements := []string{
		`CREATE INDEX IF NOT EXISTS cves_score_order_idx ON cves
      ((cvss_score + 0.2 * mention_count) DESC, first_seen DESC, cve_id ASC)`,
		`CREATE INDEX IF NOT EXISTS cves_cvss_order_idx ON cves
      (cvss_score DESC, (cvss_score + 0.2 * mention_count) DESC, first_seen DESC, cve_id ASC)`,
		`CREATE INDEX IF NOT EXISTS cves_mentions_order_idx ON cves
      (mention_count DESC, (cvss_score + 0.2 * mention_count) DESC, first_seen DESC, cve_id ASC)`,
		`CREATE INDEX IF NOT EXISTS cves_first_seen_order_idx ON cves
      (first_seen DESC, (cvss_score + 0.2 * mention_count) DESC, cve_id ASC)`,
		`CREATE TRIGGER IF NOT EXISTS cves_ranking_after_insert AFTER INSERT ON cves BEGIN
			UPDATE cve_states SET revision = revision + 1, cve_count = cve_count + 1 WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS cves_ranking_after_delete AFTER DELETE ON cves BEGIN
			UPDATE cve_states SET revision = revision + 1, cve_count = MAX(cve_count - 1, 0) WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS cves_ranking_after_update
    AFTER UPDATE OF cvss_score, first_seen, mention_count ON cves
    WHEN OLD.cvss_score IS NOT NEW.cvss_score OR OLD.first_seen IS NOT NEW.first_seen OR OLD.mention_count IS NOT NEW.mention_count
    BEGIN
      UPDATE cve_states SET revision = revision + 1 WHERE id = 1;
    END`,
		`CREATE TRIGGER IF NOT EXISTS article_cves_ranking_after_insert AFTER INSERT ON article_cves BEGIN
      UPDATE cves SET mention_count = mention_count + 1 WHERE cve_id = NEW.cve_id;
    END`,
		`CREATE TRIGGER IF NOT EXISTS article_cves_ranking_after_delete AFTER DELETE ON article_cves BEGIN
      UPDATE cves SET mention_count = MAX(mention_count - 1, 0) WHERE cve_id = OLD.cve_id;
    END`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			return fmt.Errorf("create CVE ranking schema: %w", err)
		}
	}
	return nil
}
