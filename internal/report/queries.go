package report

import (
	"context"
	"fmt"

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
