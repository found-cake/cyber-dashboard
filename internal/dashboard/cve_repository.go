package dashboard

import (
	"context"
	"errors"
	"fmt"

	"github.com/found-cake/cyber-dashboard/api"
	"gorm.io/gorm"
)

const (
	riskOrder     = "(c.cvss_score + 0.2 * c.mention_count) DESC, c.first_seen DESC, c.cve_id ASC"
	cvssOrder     = "c.cvss_score DESC, " + riskOrder
	mentionsOrder = "c.mention_count DESC, " + riskOrder
	recentOrder   = "c.first_seen DESC, " + riskOrder
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

// ErrCVEPageStale means the ranking changed after an earlier page was read.
var ErrCVEPageStale = errors.New("CVE page revision is stale")

// ErrCVEPageOutOfRange means the requested page starts after the current catalogue.
var ErrCVEPageOutOfRange = errors.New("CVE page offset is out of range")

// CVEPageRequest identifies one page within a stable server ranking.
type CVEPageRequest struct {
	Sort             CVESort
	Offset           int
	ExpectedRevision *uint64
}

// CVEPageResult carries the fixed-size values and their ranking revision.
type CVEPageResult struct {
	Values   []api.CVEInsight
	Revision uint64
}

// CVEInsights returns one fixed-size, server-sorted explorer page.
func (r *Repository) CVEInsights(ctx context.Context, request CVEPageRequest) (CVEPageResult, error) {
	result := CVEPageResult{Values: []api.CVEInsight{}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state struct {
			Revision uint64
			CVECount int64
		}
		if err := tx.Table("cve_states").Select("revision", "cve_count").Where("id = 1").Scan(&state).Error; err != nil {
			return fmt.Errorf("query CVE revision: %w", err)
		}
		result.Revision = state.Revision
		if request.ExpectedRevision != nil && *request.ExpectedRevision != result.Revision {
			return ErrCVEPageStale
		}
		if int64(request.Offset) > state.CVECount {
			return ErrCVEPageOutOfRange
		}
		values, err := queryCVEInsights(ctx, tx, cveQuery{
			order: request.Sort.order(), limit: CVEPageSize, offset: request.Offset,
		})
		if err != nil {
			return err
		}
		result.Values = values
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
	order  string
	limit  int
	offset int
}

func queryCVEInsights(ctx context.Context, db *gorm.DB, query cveQuery) ([]api.CVEInsight, error) {
	insights := []api.CVEInsight{}
	databaseQuery := db.WithContext(ctx).Table("cves AS c").Select(`c.cve_id AS id, c.cvss_score AS cvss,
		c.affected_product, c.first_seen, c.mention_count AS mentions`).Order(query.order)
	if query.limit > 0 {
		databaseQuery = databaseQuery.Limit(query.limit)
	}
	if query.offset > 0 {
		databaseQuery = databaseQuery.Offset(query.offset)
	}
	if err := databaseQuery.Scan(&insights).Error; err != nil {
		return nil, fmt.Errorf("query cve insights: %w", err)
	}
	return insights, nil
}
