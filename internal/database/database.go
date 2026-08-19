package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	sqlite "github.com/found-cake/gorm-sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dataSourceName(path)), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	hasMentionCount := db.Migrator().HasColumn(&CVE{}, "MentionCount")
	hasRankingKeys := db.Migrator().HasColumn(&CVE{}, "RiskKey")
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access sqlite pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.WithContext(ctx).AutoMigrate(&Source{}, &Article{}, &DailySummary{}, &CVE{}, &cveState{}, &RejectedCVE{}, &ArticleCVE{}, &Report{}, &Setting{}, &LLMPreset{}, &AdminCredential{}); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	if err := ensureCVERankingSchema(ctx, db, !hasMentionCount, !hasRankingKeys); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := seed(ctx, db); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	_, offsetSeconds := time.Now().Zone()
	if err := db.WithContext(ctx).Model(&Setting{}).Where("timezone_offset_minutes IS NULL").Update("timezone_offset_minutes", offsetSeconds/60).Error; err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize timezone setting: %w", err)
	}
	return db, nil
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("access sqlite pool: %w", err)
	}
	return sqlDB.Close()
}

func seed(ctx context.Context, db *gorm.DB) error {
	sources := []Source{
		{Name: "보안뉴스", Host: "boannews.com", Slug: "boannews", Enabled: false},
		{Name: "The Hacker News", Host: "thehackernews.com", Slug: "thehackernews", Enabled: true},
		{Name: "Cybersecurity News", Host: "cybersecuritynews.com", Slug: "cybersecuritynews", Enabled: true},
		{Name: "StepSecurity Blog", Host: "stepsecurity.io/blog", Slug: "stepsecurity", Enabled: true},
		{Name: "Dark Reading TI", Host: "darkreading.com/threat-intelligence", Slug: "darkreading", Enabled: true},
		{Name: "BleepingComputer", Host: "bleepingcomputer.com/news/security", Slug: "bleepingcomputer", Enabled: true},
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Select("Name", "Host", "Slug", "Enabled").Create(&sources).Error; err != nil {
		return fmt.Errorf("seed sources: %w", err)
	}
	setting := Setting{ID: 1}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&setting).Error; err != nil {
		return fmt.Errorf("seed settings: %w", err)
	}
	preset := LLMPreset{Label: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", Builtin: true}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&preset).Error; err != nil {
		return fmt.Errorf("seed LLM preset: %w", err)
	}
	return nil
}
func dataSourceName(path string) string {
	return (&url.URL{
		Scheme:   "file",
		Opaque:   (&url.URL{Path: path}).EscapedPath(),
		RawQuery: url.Values{"_pragma": {"foreign_keys(1)", "journal_mode(WAL)", "synchronous(FULL)"}}.Encode(),
	}).String()
}
