package report

import (
	"context"
	"fmt"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

type Generator interface {
	GenerateReport(ctx context.Context, request summary.ReportRequest) (summary.ReportResult, error)
}

type Service struct {
	repository *Repository
	generator  Generator
}

type CreateOptions struct {
	Language              string
	TimezoneOffsetMinutes int
}

func NewService(repository *Repository, generator Generator) *Service {
	return &Service{repository: repository, generator: generator}
}

func (s *Service) Create(ctx context.Context, request api.CreateReportRequest, options CreateOptions) (api.Report, error) {
	draft, err := s.repository.Build(ctx, request)
	if err != nil {
		return api.Report{}, err
	}
	value := draft.report
	facts := draft.facts
	facts = append(facts,
		fmt.Sprintf("period=%s..%s", request.Start, request.End),
		fmt.Sprintf("total=%d critical=%d high=%d medium=%d", value.Total, value.Critical, value.High, value.Medium),
	)
	candidates := make([]summary.ReportThreatCandidate, len(draft.threats))
	for index, threat := range draft.threats {
		candidates[index] = threat.summaryCandidate()
	}
	result, err := s.generator.GenerateReport(ctx, summary.ReportRequest{
		Language: options.Language, Kind: request.Type + " report", Facts: facts,
		Threats: candidates, ThreatLimit: topThreatLimit(request.Type),
	})
	if err != nil {
		return api.Report{}, fmt.Errorf("generate report summary: %w", err)
	}
	value.Summary = result.Summary
	value.TopThreats = selectedReportThreats(draft.threats, result.ThreatGroups, topThreatLimit(request.Type))
	value.TopThreat = firstThreatTitle(value.TopThreats)
	return s.repository.Save(ctx, value, options.TimezoneOffsetMinutes)
}
