package report

import (
	"context"
	"fmt"
	"slices"
	"sort"
)

type threatArticle struct {
	ID            int64
	Title         string
	URL           string
	Summary       string
	Severity      string
	PublishedAt   string
	PublishedTime string
	VictimCount   int
	DamageUSD     int64
	ZeroDay       bool
	CVEs          string `gorm:"column:cves"`
}

type threatCandidate struct {
	id          string
	title       string
	summary     string
	severity    string
	publishedAt string
	sourceCount int
	cves        []string
	victimCount int
	damageUSD   int64
	zeroDay     bool
	articleID   int64
}

func topThreatLimit(reportType string) int {
	if reportType == "monthly" {
		return 10
	}
	if reportType == "weekly" {
		return 3
	}
	return 1
}

func (r *Repository) threatCandidates(ctx context.Context, period valuePeriod, maximum int) ([]threatCandidate, error) {
	var rows []threatArticle
	err := r.db.WithContext(ctx).Raw(`SELECT a.id, a.title, a.url, a.summary, a.severity, a.published_at,
		a.published_time, a.victim_count, a.damage_usd, a.zero_day,
		COALESCE(GROUP_CONCAT(ac.cve_id), '') AS cves
		FROM articles AS a LEFT JOIN article_cves AS ac ON ac.article_id = a.id
		WHERE a.published_at BETWEEN ? AND ? GROUP BY a.id`, period.start, period.end).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query report threats: %w", err)
	}
	groups := make([][]threatArticle, 0, len(rows))
	for _, row := range rows {
		groupIndex := matchingThreatGroup(groups, row)
		if groupIndex < 0 {
			groups = append(groups, []threatArticle{row})
			continue
		}
		groups[groupIndex] = append(groups[groupIndex], row)
	}
	candidates := make([]threatCandidate, 0, len(groups))
	for _, group := range groups {
		candidate := newThreatCandidate(group)
		if candidate.severity == "CRITICAL" {
			candidates = append(candidates, candidate)
		}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return threatCandidateBefore(candidates[left], candidates[right])
	})
	if len(candidates) > maximum {
		candidates = candidates[:maximum]
	}
	for index := range candidates {
		candidates[index].id = fmt.Sprintf("threat-%d", index+1)
	}
	return candidates, nil
}

func matchingThreatGroup(groups [][]threatArticle, candidate threatArticle) int {
	for groupIndex, group := range groups {
		for _, member := range group {
			if sameThreat(member, candidate) {
				return groupIndex
			}
		}
	}
	return -1
}

func newThreatCandidate(group []threatArticle) threatCandidate {
	representative := group[0]
	cves := map[string]struct{}{}
	maximumVictims, maximumDamage := representative.VictimCount, representative.DamageUSD
	zeroDay := representative.ZeroDay
	for _, article := range group {
		if threatArticleBefore(article, representative) {
			representative = article
		}
		maximumVictims = max(maximumVictims, article.VictimCount)
		maximumDamage = max(maximumDamage, article.DamageUSD)
		zeroDay = zeroDay || article.ZeroDay
		for _, cve := range splitCVEs(article.CVEs) {
			cves[cve] = struct{}{}
		}
	}
	orderedCVEs := make([]string, 0, len(cves))
	for cve := range cves {
		orderedCVEs = append(orderedCVEs, cve)
	}
	slices.Sort(orderedCVEs)
	return threatCandidate{
		title: representative.Title, summary: representative.Summary, severity: representative.Severity,
		publishedAt: representative.PublishedAt, sourceCount: len(group), cves: orderedCVEs,
		victimCount: maximumVictims, damageUSD: maximumDamage, zeroDay: zeroDay, articleID: representative.ID,
	}
}

func threatArticleBefore(left, right threatArticle) bool {
	leftCandidate := threatCandidate{severity: left.Severity, publishedAt: left.PublishedAt + left.PublishedTime,
		victimCount: left.VictimCount, damageUSD: left.DamageUSD, zeroDay: left.ZeroDay, articleID: left.ID}
	rightCandidate := threatCandidate{severity: right.Severity, publishedAt: right.PublishedAt + right.PublishedTime,
		victimCount: right.VictimCount, damageUSD: right.DamageUSD, zeroDay: right.ZeroDay, articleID: right.ID}
	return threatCandidateBefore(leftCandidate, rightCandidate)
}

func threatCandidateBefore(left, right threatCandidate) bool {
	if severityPriority(left.severity) != severityPriority(right.severity) {
		return severityPriority(left.severity) < severityPriority(right.severity)
	}
	if left.damageUSD != right.damageUSD {
		return left.damageUSD > right.damageUSD
	}
	if left.victimCount != right.victimCount {
		return left.victimCount > right.victimCount
	}
	if left.zeroDay != right.zeroDay {
		return left.zeroDay
	}
	if left.sourceCount != right.sourceCount {
		return left.sourceCount > right.sourceCount
	}
	if left.publishedAt != right.publishedAt {
		return left.publishedAt > right.publishedAt
	}
	return left.articleID < right.articleID
}

func severityPriority(value string) int {
	switch value {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	case "LOW":
		return 3
	default:
		return 4
	}
}
