package enrichment

import (
	"context"
	"errors"
	"fmt"

	"github.com/found-cake/cyber-dashboard/internal/feed/store"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

type ArticleAnalyzer interface {
	AnalyzeArticle(ctx context.Context, request summary.ArticleRequest) (summary.ArticleAnalysis, error)
}

type Repository interface {
	ArticlesForAnalysis(ctx context.Context, day string) ([]store.ArticleForAnalysis, error)
	SaveArticleAnalysis(ctx context.Context, articleID int64, analysis summary.ArticleAnalysis) error
}

type ArticleEnrichmentService struct {
	repository Repository
	analyzer   ArticleAnalyzer
}

func NewArticleEnrichmentService(repository Repository, analyzer ArticleAnalyzer) *ArticleEnrichmentService {
	return &ArticleEnrichmentService{repository: repository, analyzer: analyzer}
}

func (s *ArticleEnrichmentService) EnrichDay(ctx context.Context, day, language string) error {
	articles, err := s.repository.ArticlesForAnalysis(ctx, day)
	if err != nil {
		return err
	}
	failures := []error{}
	for _, article := range articles {
		analysis, analyzeErr := s.analyzer.AnalyzeArticle(ctx, summary.ArticleRequest{
			Language: language, Title: article.Title, URL: article.URL, Body: article.Body,
		})
		if analyzeErr != nil {
			failures = append(failures, fmt.Errorf("analyze article %d: %w", article.ID, analyzeErr))
			continue
		}
		if saveErr := s.repository.SaveArticleAnalysis(ctx, article.ID, analysis); saveErr != nil {
			failures = append(failures, saveErr)
		}
	}
	return errors.Join(failures...)
}
