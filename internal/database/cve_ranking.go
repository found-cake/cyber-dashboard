package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ensureCVERankingSchema(ctx context.Context, db *gorm.DB, backfillMentions, backfillKeys bool) error {
	if backfillMentions {
		if err := db.WithContext(ctx).Exec(`UPDATE cves SET mention_count =
      (SELECT COUNT(*) FROM article_cves WHERE article_cves.cve_id = cves.cve_id)`).Error; err != nil {
			return fmt.Errorf("backfill CVE mention counts: %w", err)
		}
	}
	if backfillMentions || backfillKeys {
		if err := db.WithContext(ctx).Exec(`UPDATE cves SET
      risk_key = -(cvss_score + 0.2 * mention_count), cvss_key = -cvss_score,
      mentions_key = -mention_count, first_seen_key = -CAST(REPLACE(first_seen, '-', '') AS INTEGER)`).Error; err != nil {
			return fmt.Errorf("backfill CVE ranking keys: %w", err)
		}
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&cveState{ID: 1, Revision: 1}).Error; err != nil {
		return fmt.Errorf("initialize CVE revision: %w", err)
	}
	statements := []string{
		`DROP INDEX IF EXISTS cves_score_order_idx`,
		`DROP INDEX IF EXISTS cves_cvss_order_idx`,
		`DROP INDEX IF EXISTS cves_mentions_order_idx`,
		`DROP INDEX IF EXISTS cves_first_seen_order_idx`,
		`DROP INDEX IF EXISTS cves_score_cursor_idx`,
		`DROP INDEX IF EXISTS cves_cvss_cursor_idx`,
		`DROP INDEX IF EXISTS cves_mentions_cursor_idx`,
		`DROP INDEX IF EXISTS cves_first_seen_cursor_idx`,
		`CREATE INDEX IF NOT EXISTS cves_score_seek_idx ON cves (risk_key, first_seen_key, cve_id)`,
		`CREATE INDEX IF NOT EXISTS cves_cvss_seek_idx ON cves (cvss_key, risk_key, first_seen_key, cve_id)`,
		`CREATE INDEX IF NOT EXISTS cves_mentions_seek_idx ON cves (mentions_key, risk_key, first_seen_key, cve_id)`,
		`CREATE INDEX IF NOT EXISTS cves_first_seen_seek_idx ON cves (first_seen_key, risk_key, cve_id)`,
		`DROP TRIGGER IF EXISTS cves_ranking_after_insert`,
		`DROP TRIGGER IF EXISTS cves_ranking_after_delete`,
		`DROP TRIGGER IF EXISTS cves_ranking_after_update`,
		`CREATE TRIGGER IF NOT EXISTS cves_ranking_after_insert AFTER INSERT ON cves BEGIN
			UPDATE cves SET risk_key = -(NEW.cvss_score + 0.2 * NEW.mention_count), cvss_key = -NEW.cvss_score,
				mentions_key = -NEW.mention_count, first_seen_key = -CAST(REPLACE(NEW.first_seen, '-', '') AS INTEGER)
				WHERE cve_id = NEW.cve_id;
			UPDATE cve_states SET revision = revision + 1 WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS cves_ranking_after_delete AFTER DELETE ON cves BEGIN
			UPDATE cve_states SET revision = revision + 1 WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS cves_ranking_after_update
    AFTER UPDATE OF cvss_score, first_seen, mention_count ON cves
    WHEN OLD.cvss_score IS NOT NEW.cvss_score OR OLD.first_seen IS NOT NEW.first_seen OR OLD.mention_count IS NOT NEW.mention_count
    BEGIN
		UPDATE cves SET risk_key = -(NEW.cvss_score + 0.2 * NEW.mention_count), cvss_key = -NEW.cvss_score,
			mentions_key = -NEW.mention_count, first_seen_key = -CAST(REPLACE(NEW.first_seen, '-', '') AS INTEGER)
			WHERE cve_id = NEW.cve_id;
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
