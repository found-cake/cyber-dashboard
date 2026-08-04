package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

var ErrNotFound = errors.New("report data not found")

type Repository struct {
	db  *sql.DB
	now func() time.Time
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, now: time.Now}
}

func (r *Repository) List(ctx context.Context) ([]api.Report, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, type, period_start, period_end, total,
	critical, high, medium, top_threat, actors, sectors, summary, generated_at FROM reports ORDER BY generated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	defer rows.Close()
	reports := []api.Report{}
	for rows.Next() {
		var value api.Report
		var actors, sectors string
		if err := rows.Scan(&value.ID, &value.Type, &value.PeriodStart, &value.PeriodEnd,
			&value.Total, &value.Critical, &value.High, &value.Medium, &value.TopThreat,
			&actors, &sectors, &value.Summary, &value.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		value.Actors = decodeList(actors)
		value.Sectors = decodeList(sectors)
		reports = append(reports, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reports: %w", err)
	}
	return reports, nil
}

func (r *Repository) Build(ctx context.Context, request api.CreateReportRequest) (api.Report, []string, error) {
	value := api.Report{Type: request.Type, PeriodStart: request.Start, PeriodEnd: request.End, Actors: []string{}, Sectors: []string{}}
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(severity = 'CRITICAL'), 0),
    COALESCE(SUM(severity = 'HIGH'), 0), COALESCE(SUM(severity = 'MEDIUM'), 0)
    FROM articles WHERE published_at BETWEEN ? AND ?`, request.Start, request.End).Scan(
		&value.Total, &value.Critical, &value.High, &value.Medium)
	if err != nil {
		return api.Report{}, nil, fmt.Errorf("aggregate report: %w", err)
	}
	if value.Total == 0 {
		return api.Report{}, nil, fmt.Errorf("report %s to %s: %w", request.Start, request.End, ErrNotFound)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT title FROM articles WHERE published_at BETWEEN ? AND ?
    ORDER BY CASE severity WHEN 'CRITICAL' THEN 0 WHEN 'HIGH' THEN 1 ELSE 2 END, published_at DESC LIMIT 1`,
		request.Start, request.End).Scan(&value.TopThreat); err != nil {
		return api.Report{}, nil, fmt.Errorf("query top threat: %w", err)
	}
	period := valuePeriod{start: request.Start, end: request.End}
	value.Actors, err = r.topValues(ctx, topQuery{column: "threat_actor", period: period, limit: 6})
	if err != nil {
		return api.Report{}, nil, err
	}
	value.Sectors, err = r.topValues(ctx, topQuery{column: "sector", period: period, limit: 5})
	if err != nil {
		return api.Report{}, nil, err
	}
	facts, err := r.articleFacts(ctx, period)
	if err != nil {
		return api.Report{}, nil, err
	}
	return value, facts, nil
}

func (r *Repository) Save(ctx context.Context, value api.Report, timezoneOffsetMinutes int) (api.Report, error) {
	location := time.FixedZone("configured", timezoneOffsetMinutes*60)
	value.GeneratedAt = r.now().In(location).Format(time.RFC3339)
	actors, err := encodeList(value.Actors)
	if err != nil {
		return api.Report{}, err
	}
	sectors, err := encodeList(value.Sectors)
	if err != nil {
		return api.Report{}, err
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO reports (type, period_start, period_end, total,
    critical, high, medium, top_threat, actors, sectors, summary, generated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.Type, value.PeriodStart, value.PeriodEnd,
		value.Total, value.Critical, value.High, value.Medium, value.TopThreat, actors,
		sectors, value.Summary, value.GeneratedAt)
	if err != nil {
		return api.Report{}, fmt.Errorf("insert report: %w", err)
	}
	value.ID, err = result.LastInsertId()
	if err != nil {
		return api.Report{}, fmt.Errorf("report id: %w", err)
	}
	return value, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM reports WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete report %d: %w", id, err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("deleted report count %d: %w", id, err)
	}
	if deleted == 0 {
		return fmt.Errorf("report %d: %w", id, ErrNotFound)
	}
	return nil
}

// encodeList stores a string list as a JSON array. Actor and sector names come from LLM
// analysis of article text and routinely contain commas ("Scattered Spider, Inc.",
// "금융, 보험"), which a comma-joined column cannot round-trip.
func encodeList(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode report list: %w", err)
	}
	return string(encoded), nil
}

// decodeList reads the JSON array written by encodeList, falling back to the original
// comma-separated format so reports stored before that change still load.
func decodeList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{}
	}
	if strings.HasPrefix(trimmed, "[") {
		var values []string
		if err := json.Unmarshal([]byte(trimmed), &values); err == nil {
			if values == nil {
				return []string{}
			}
			return values
		}
	}
	parts := strings.Split(trimmed, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, strings.TrimSpace(part))
	}
	return values
}
