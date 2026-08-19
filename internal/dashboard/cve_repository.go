package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"gorm.io/gorm"
)

const (
	riskKeyExpression      = "c.risk_key"
	cvssKeyExpression      = "c.cvss_key"
	mentionsKeyExpression  = "c.mentions_key"
	firstSeenKeyExpression = "c.first_seen_key"
	riskOrder              = riskKeyExpression + " ASC, " + firstSeenKeyExpression + " ASC, c.cve_id ASC"
	cvssOrder              = cvssKeyExpression + " ASC, " + riskOrder
	mentionsOrder          = mentionsKeyExpression + " ASC, " + riskOrder
	recentOrder            = firstSeenKeyExpression + " ASC, " + riskKeyExpression + " ASC, c.cve_id ASC"
	maxCVECursorLength     = 128
)

// CVESort is a server-owned CVE explorer ranking.
type CVESort string

const (
	CVESortScore     CVESort = "score"
	CVESortCVSS      CVESort = "cvss"
	CVESortMentions  CVESort = "mentions"
	CVESortFirstSeen CVESort = "firstSeen"
)

// ParseCVESort converts the public query value into a supported ranking.
func ParseCVESort(value string) (CVESort, bool) {
	sort := CVESort(value)
	switch sort {
	case CVESortScore, CVESortCVSS, CVESortMentions, CVESortFirstSeen:
		return sort, true
	default:
		return "", false
	}
}

func (s CVESort) order() string {
	switch s {
	case CVESortScore:
		return riskOrder
	case CVESortCVSS:
		return cvssOrder
	case CVESortMentions:
		return mentionsOrder
	case CVESortFirstSeen:
		return recentOrder
	default:
		return riskOrder
	}
}

var (
	// ErrCVEPageStale means the ranking changed after an earlier page was read.
	ErrCVEPageStale = errors.New("CVE page revision is stale")
	// ErrCVECursorInvalid means the continuation does not identify a row in this ranking.
	ErrCVECursorInvalid = errors.New("CVE page cursor is invalid")
)

// CVEPageRequest identifies one page within a stable server ranking.
type CVEPageRequest struct {
	Sort             CVESort
	Cursor           string
	ExpectedRevision *uint64
}

// CVEPageResult carries the fixed-size values and their ranking revision.
type CVEPageResult struct {
	Values     []api.CVEInsight
	Revision   uint64
	NextCursor string
}

// CVEInsights returns one fixed-size, server-sorted explorer page.
func (r *Repository) CVEInsights(ctx context.Context, request CVEPageRequest) (CVEPageResult, error) {
	result := CVEPageResult{Values: []api.CVEInsight{}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("cve_states").Select("revision").Where("id = 1").Scan(&result.Revision).Error; err != nil {
			return fmt.Errorf("query CVE revision: %w", err)
		}
		if request.ExpectedRevision != nil && *request.ExpectedRevision != result.Revision {
			return ErrCVEPageStale
		}
		query := cveQuery{order: request.Sort.order(), limit: CVEPageSize}
		if request.Cursor != "" {
			cursorID, ok := request.Sort.cursorID(request.Cursor)
			if !ok {
				return ErrCVECursorInvalid
			}
			cursor, err := loadCVECursor(ctx, tx, cursorID)
			if err != nil {
				return err
			}
			query.where, query.arguments = request.Sort.seek(cursor)
		}
		values, err := queryCVEInsights(ctx, tx, query)
		if err != nil {
			return err
		}
		result.Values = values
		if len(values) == CVEPageSize {
			result.NextCursor = request.Sort.cursor(values[len(values)-1].ID)
		}
		return nil
	})
	if err != nil {
		return CVEPageResult{}, err
	}
	return result, nil
}

func (r *Repository) recentCVEs(ctx context.Context, limit int) ([]api.CVEInsight, error) {
	return queryCVEInsights(ctx, r.db, cveQuery{order: recentOrder, limit: limit})
}

type cveQuery struct {
	order     string
	where     string
	arguments []any
	limit     int
}

func queryCVEInsights(ctx context.Context, db *gorm.DB, query cveQuery) ([]api.CVEInsight, error) {
	insights := []api.CVEInsight{}
	databaseQuery := db.WithContext(ctx).Table("cves AS c").Select(`c.cve_id AS id, c.cvss_score AS cvss,
		c.affected_product, c.first_seen, c.mention_count AS mentions`).Order(query.order)
	if query.where != "" {
		databaseQuery = databaseQuery.Where(query.where, query.arguments...)
	}
	if query.limit > 0 {
		databaseQuery = databaseQuery.Limit(query.limit)
	}
	if err := databaseQuery.Scan(&insights).Error; err != nil {
		return nil, fmt.Errorf("query cve insights: %w", err)
	}
	return insights, nil
}

type cveCursor struct {
	RiskKey      float64
	CVSSKey      float64
	MentionsKey  int
	FirstSeenKey int
	ID           string
}

func loadCVECursor(ctx context.Context, db *gorm.DB, id string) (cveCursor, error) {
	var cursor cveCursor
	err := db.WithContext(ctx).Table("cves AS c").Select(riskKeyExpression+` AS risk_key, `+
		cvssKeyExpression+` AS cvss_key, `+mentionsKeyExpression+` AS mentions_key, `+
		firstSeenKeyExpression+` AS first_seen_key, c.cve_id AS id`).Where("c.cve_id = ?", id).Take(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return cveCursor{}, ErrCVECursorInvalid
	}
	if err != nil {
		return cveCursor{}, fmt.Errorf("query CVE cursor: %w", err)
	}
	return cursor, nil
}

func (s CVESort) cursor(id string) string {
	return string(s) + "." + id
}

func (s CVESort) cursorID(value string) (string, bool) {
	sort, id, found := strings.Cut(value, ".")
	return id, found && sort == string(s) && id != "" && len(value) <= maxCVECursorLength
}

func (s CVESort) seek(cursor cveCursor) (string, []any) {
	switch s {
	case CVESortCVSS:
		return "(" + cvssKeyExpression + ", " + riskKeyExpression + ", " + firstSeenKeyExpression + ", c.cve_id) > (?, ?, ?, ?)",
			[]any{cursor.CVSSKey, cursor.RiskKey, cursor.FirstSeenKey, cursor.ID}
	case CVESortMentions:
		return "(" + mentionsKeyExpression + ", " + riskKeyExpression + ", " + firstSeenKeyExpression + ", c.cve_id) > (?, ?, ?, ?)",
			[]any{cursor.MentionsKey, cursor.RiskKey, cursor.FirstSeenKey, cursor.ID}
	case CVESortFirstSeen:
		return "(" + firstSeenKeyExpression + ", " + riskKeyExpression + ", c.cve_id) > (?, ?, ?)",
			[]any{cursor.FirstSeenKey, cursor.RiskKey, cursor.ID}
	default:
		return "(" + riskKeyExpression + ", " + firstSeenKeyExpression + ", c.cve_id) > (?, ?, ?)",
			[]any{cursor.RiskKey, cursor.FirstSeenKey, cursor.ID}
	}
}
