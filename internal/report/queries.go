package report

import (
	"context"
	"fmt"
	"strings"

	"github.com/found-cake/cyber-dashboard/internal/database"
)

type valuePeriod struct {
	start string
	end   string
}

type topQuery struct {
	column string
	period valuePeriod
	limit  int
}

func (r *Repository) topValues(ctx context.Context, query topQuery) ([]string, error) {
	switch query.column {
	case "threat_actor":
	case "sector":
	default:
		return nil, fmt.Errorf("top values column %q: invalid", query.column)
	}
	values := []string{}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).Where("published_at BETWEEN ? AND ?", query.period.start, query.period.end).
		Where(query.column+" != ''").Group(query.column).Order("COUNT(*) DESC").Limit(query.limit).Pluck(query.column, &values).Error; err != nil {
		return nil, fmt.Errorf("query top %s: %w", query.column, err)
	}
	return values, nil
}

func (r *Repository) dailySummaryFacts(ctx context.Context, period valuePeriod) ([]string, error) {
	var rows []struct {
		Day     string
		Summary string
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT days.day, COALESCE(s.summary, '') AS summary
		FROM (SELECT DISTINCT published_at AS day FROM articles WHERE published_at BETWEEN ? AND ?) AS days
		LEFT JOIN daily_summaries AS s ON s.day = days.day ORDER BY days.day`, period.start, period.end).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query report daily summaries: %w", err)
	}
	facts := make([]string, 0, len(rows))
	for _, row := range rows {
		summary := strings.TrimSpace(row.Summary)
		if summary == "" {
			return nil, fmt.Errorf("daily summary %s: %w", row.Day, ErrDailySummariesRequired)
		}
		facts = append(facts, row.Day+": "+summary)
	}
	return facts, nil
}

func (r *Repository) articleFacts(ctx context.Context, period valuePeriod) ([]string, error) {
	var rows []struct {
		Title   string
		Summary string
	}
	if err := r.db.WithContext(ctx).Model(&database.Article{}).Select("title", "summary").
		Where("published_at BETWEEN ? AND ?", period.start, period.end).Order("published_at DESC").Limit(30).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query report facts: %w", err)
	}
	facts := make([]string, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, row.Title+": "+row.Summary)
	}
	return facts, nil
}
