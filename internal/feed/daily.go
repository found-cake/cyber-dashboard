package feed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) Daily(ctx context.Context, day string) (api.Daily, error) {
	var rows []struct {
		ID           int64
		Source       string
		FeedUID      string
		Title        string
		URL          string
		PublishedAt  string
		Body         string
		Summary      string
		AttackMethod string
		ThreatActor  string
		ActorCountry string
		Sector       string
		VictimCount  int
		ZeroDay      bool
		Severity     string
	}
	daily := api.Daily{Day: day, Articles: []api.Article{}}
	if err := r.db.WithContext(ctx).Table("articles AS a").Select(`a.id, s.name AS source, a.feed_uid, a.title, a.url,
	COALESCE(NULLIF(a.published_time, ''), a.published_at) AS published_at, a.body, a.summary, a.attack_method,
	a.threat_actor, a.actor_country, a.sector, a.victim_count, a.zero_day, a.severity`).
		Joins("JOIN sources AS s ON s.id = a.source_id").Where("a.published_at = ?", day).
		Order("a.published_time DESC, a.id DESC").Scan(&rows).Error; err != nil {
		return api.Daily{}, fmt.Errorf("query daily articles: %w", err)
	}
	for _, row := range rows {
		daily.Articles = append(daily.Articles, api.Article{ID: row.ID, Source: row.Source, FeedUID: row.FeedUID,
			Title: row.Title, URL: row.URL, PublishedAt: row.PublishedAt, Body: row.Body, Summary: row.Summary,
			AttackMethod: row.AttackMethod, ThreatActor: row.ThreatActor, ActorCountry: row.ActorCountry,
			Sector: row.Sector, VictimCount: row.VictimCount, ZeroDay: row.ZeroDay, Severity: row.Severity})
	}
	var err error
	for index := range daily.Articles {
		daily.Articles[index].CVEs, err = r.articleCVEs(ctx, daily.Articles[index].ID)
		if err != nil {
			return api.Daily{}, err
		}
	}
	err = r.db.WithContext(ctx).Model(&database.DailySummary{}).Select("summary").Where("day = ?", day).Scan(&daily.Summary).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return api.Daily{}, fmt.Errorf("query daily summary: %w", err)
	}
	return daily, nil
}

func (r *Repository) SaveDailySummary(ctx context.Context, day, value string) error {
	stored := database.DailySummary{Day: day, Summary: value, GeneratedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "day"}}, DoUpdates: clause.AssignmentColumns([]string{"summary", "generated_at"})}).Create(&stored).Error; err != nil {
		return fmt.Errorf("save daily summary: %w", err)
	}
	return nil
}

func (r *Repository) articleCVEs(ctx context.Context, articleID int64) ([]string, error) {
	cves := []string{}
	if err := r.db.WithContext(ctx).Model(&database.ArticleCVE{}).Where("article_id = ?", articleID).Order("cve_id").Pluck("cve_id", &cves).Error; err != nil {
		return nil, fmt.Errorf("query article cves: %w", err)
	}
	return cves, nil
}

func (r *Repository) CollectedDays(ctx context.Context) ([]string, error) {
	days := []string{}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).Distinct("published_at").Where("published_at != ''").Order("published_at").Pluck("published_at", &days).Error; err != nil {
		return nil, fmt.Errorf("query collected days: %w", err)
	}
	return days, nil
}
