package summary

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/utils"
)

var ErrNotConfigured = errors.New("LLM endpoint is not configured")
var ErrUnavailable = errors.New("LLM endpoint is unavailable")

const maximumSummaryFacts = 5

// maximumStandaloneParagraphs limits direct paragraph joins before a merge pass rewrites
// the batches into one digest.
const maximumStandaloneParagraphs = 2

type SettingsProvider interface {
	Get(ctx context.Context) (api.Settings, error)
}

type Service struct {
	settings SettingsProvider
}

func NewService(provider SettingsProvider) *Service {
	return &Service{settings: provider}
}

func (s *Service) Generate(ctx context.Context, request Request) (string, error) {
	client, err := s.client(ctx)
	if err != nil {
		return "", err
	}
	if len(request.Facts) <= maximumSummaryFacts {
		value, generateErr := utils.RetryOnceOnError(ErrInvalidResponse, func() (string, error) {
			return client.Generate(ctx, request)
		})
		if generateErr != nil {
			return "", fmt.Errorf("%w: %w", ErrUnavailable, generateErr)
		}
		return value, nil
	}
	batchCount := (len(request.Facts) + maximumSummaryFacts - 1) / maximumSummaryFacts
	mergeBatches := batchCount > maximumStandaloneParagraphs
	parts := make([]string, 0, batchCount)
	for start := 0; start < len(request.Facts); start += maximumSummaryFacts {
		end := min(start+maximumSummaryFacts, len(request.Facts))
		batch := request
		batch.Facts = request.Facts[start:end]
		batch.section = mergeBatches
		value, generateErr := utils.RetryOnceOnError(ErrInvalidResponse, func() (string, error) {
			return client.Generate(ctx, batch)
		})
		if generateErr != nil {
			return "", fmt.Errorf("%w: summarize batch %d: %w", ErrUnavailable, start/maximumSummaryFacts+1, generateErr)
		}
		parts = append(parts, value)
	}
	joined := strings.Join(parts, "\n\n")
	if !mergeBatches {
		return joined, nil
	}
	value, err := utils.RetryOnceOnError(ErrInvalidResponse, func() (string, error) {
		return client.merge(ctx, mergeRequest{Language: request.Language, Kind: request.Kind, Sections: parts})
	})
	if err != nil {
		// Preserve complete sections when the readability-only merge fails.
		slog.WarnContext(ctx, "summary merge failed", slog.String("response_body", err.Error()))
		return joined, nil
	}
	return value, nil
}

func (s *Service) AnalyzeArticle(ctx context.Context, request ArticleRequest) (ArticleAnalysis, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ArticleAnalysis{}, err
	}
	value, err := utils.RetryOnceOnError(ErrInvalidResponse, func() (ArticleAnalysis, error) {
		return client.AnalyzeArticle(ctx, request)
	})
	if err != nil {
		return ArticleAnalysis{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return pairActorWithMethod(value), nil
}

// pairActorWithMethod keeps attack and actor classifications consistent. It remains in the
// service so promptbench can measure the unnormalized model response through the client.
func pairActorWithMethod(analysis ArticleAnalysis) ArticleAnalysis {
	if !IsIncidentMethod(analysis.AttackMethod) {
		analysis.ThreatActor = noAttackActor
		analysis.ActorCountry = ""
		return analysis
	}
	if analysis.ThreatActor == noAttackActor {
		analysis.ThreatActor = unknownActor
	}
	return analysis
}

func (s *Service) TestConnection(ctx context.Context) error {
	value, err := s.settings.Get(ctx)
	if err != nil {
		return fmt.Errorf("load LLM settings: %w", err)
	}
	return s.TestConnectionWithSettings(ctx, value)
}

func (s *Service) TestConnectionWithSettings(ctx context.Context, value api.Settings) error {
	client, err := clientFromSettings(value)
	if err != nil {
		return err
	}
	if err := client.TestConnection(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

func (s *Service) client(ctx context.Context) (*Client, error) {
	value, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load LLM settings: %w", err)
	}
	return clientFromSettings(value)
}

func clientFromSettings(value api.Settings) (*Client, error) {
	parsed, err := url.Parse(value.LLMBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse LLM base URL: %w", err)
	}
	if value.LLMAPIKey == "" && parsed.Hostname() == "api.openai.com" {
		return nil, ErrNotConfigured
	}
	return NewClient(Config{
		BaseURL: value.LLMBaseURL,
		Model:   value.LLMModel,
		APIKey:  value.LLMAPIKey,
		Timeout: time.Duration(value.LLMTimeout) * time.Second,
	})
}
