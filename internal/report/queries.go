package report

import (
	"context"
	"fmt"
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
	var statement string
	switch query.column {
	case "threat_actor":
		statement = `SELECT threat_actor FROM articles WHERE published_at BETWEEN ? AND ?
    AND threat_actor != '' GROUP BY threat_actor ORDER BY COUNT(*) DESC LIMIT ?`
	case "sector":
		statement = `SELECT sector FROM articles WHERE published_at BETWEEN ? AND ?
    AND sector != '' GROUP BY sector ORDER BY COUNT(*) DESC LIMIT ?`
	default:
		return nil, fmt.Errorf("top values column %q: invalid", query.column)
	}
	rows, err := r.db.QueryContext(ctx, statement, query.period.start, query.period.end, query.limit)
	if err != nil {
		return nil, fmt.Errorf("query top %s: %w", query.column, err)
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan top %s: %w", query.column, err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top %s: %w", query.column, err)
	}
	return values, nil
}

func (r *Repository) articleFacts(ctx context.Context, period valuePeriod) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT title, summary FROM articles
    WHERE published_at BETWEEN ? AND ? ORDER BY published_at DESC LIMIT 30`, period.start, period.end)
	if err != nil {
		return nil, fmt.Errorf("query report facts: %w", err)
	}
	defer rows.Close()
	facts := []string{}
	for rows.Next() {
		var title, articleSummary string
		if err := rows.Scan(&title, &articleSummary); err != nil {
			return nil, fmt.Errorf("scan report fact: %w", err)
		}
		facts = append(facts, title+": "+articleSummary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report facts: %w", err)
	}
	return facts, nil
}
