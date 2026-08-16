package report

import (
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

func (candidate threatCandidate) summaryCandidate() summary.ReportThreatCandidate {
	return summary.ReportThreatCandidate{
		ID: candidate.id, Title: candidate.title, Summary: candidate.summary, Severity: candidate.severity,
		PublishedAt: candidate.publishedAt, SourceCount: candidate.sourceCount, CVEs: candidate.cves,
	}
}

func (candidate threatCandidate) reportThreat() api.ReportThreat {
	return api.ReportThreat{
		Title: candidate.title, Severity: candidate.severity,
		PublishedAt: candidate.publishedAt, SourceCount: candidate.sourceCount,
	}
}

func staticReportThreats(candidates []threatCandidate, limit int) []api.ReportThreat {
	count := min(len(candidates), limit)
	result := make([]api.ReportThreat, count)
	for index := range result {
		result[index] = candidates[index].reportThreat()
	}
	return result
}

func selectedReportThreats(candidates []threatCandidate, groups []summary.ReportThreatGroup, limit int) []api.ReportThreat {
	if len(groups) == 0 || len(groups) > limit {
		return staticReportThreats(candidates, limit)
	}
	byID := make(map[string]threatCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.id] = candidate
	}
	used := map[string]struct{}{}
	result := make([]api.ReportThreat, 0, len(groups))
	for _, group := range groups {
		representative, ok := byID[group.RepresentativeID]
		translatedTitle := strings.TrimSpace(group.TranslatedTitle)
		if !ok || len(group.MemberIDs) == 0 || translatedTitle == "" {
			return staticReportThreats(candidates, limit)
		}
		sourceCount, representativeIncluded := 0, false
		for _, id := range group.MemberIDs {
			member, exists := byID[id]
			_, repeated := used[id]
			if !exists || repeated {
				return staticReportThreats(candidates, limit)
			}
			used[id] = struct{}{}
			sourceCount += member.sourceCount
			representativeIncluded = representativeIncluded || id == group.RepresentativeID
		}
		if !representativeIncluded {
			return staticReportThreats(candidates, limit)
		}
		threat := representative.reportThreat()
		threat.Title = translatedTitle
		threat.SourceCount = sourceCount
		result = append(result, threat)
	}
	return result
}

func firstThreatTitle(threats []api.ReportThreat) string {
	if len(threats) == 0 {
		return ""
	}
	return threats[0].Title
}
