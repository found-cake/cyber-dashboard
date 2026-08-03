package summary

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

var ErrNotConfigured = errors.New("LLM endpoint is not configured")
var ErrUnavailable = errors.New("LLM endpoint is unavailable")

const maximumSummaryFacts = 5

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
		value, generateErr := generateWithRetry(ctx, client, request)
		if generateErr != nil {
			return "", fmt.Errorf("%w: %w", ErrUnavailable, generateErr)
		}
		return value, nil
	}
	paragraphs := make([]string, 0, (len(request.Facts)+maximumSummaryFacts-1)/maximumSummaryFacts)
	for start := 0; start < len(request.Facts); start += maximumSummaryFacts {
		end := min(start+maximumSummaryFacts, len(request.Facts))
		batch := request
		batch.Facts = request.Facts[start:end]
		value, generateErr := generateWithRetry(ctx, client, batch)
		if generateErr != nil {
			return "", fmt.Errorf("%w: summarize batch %d: %w", ErrUnavailable, start/maximumSummaryFacts+1, generateErr)
		}
		paragraphs = append(paragraphs, value)
	}
	return strings.Join(paragraphs, "\n\n"), nil
}

func generateWithRetry(ctx context.Context, client *Client, request Request) (string, error) {
	value, err := client.Generate(ctx, request)
	if errors.Is(err, ErrInvalidResponse) {
		return client.Generate(ctx, request)
	}
	return value, err
}

func (s *Service) AnalyzeArticle(ctx context.Context, request ArticleRequest) (ArticleAnalysis, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ArticleAnalysis{}, err
	}
	value, err := client.AnalyzeArticle(ctx, request)
	if errors.Is(err, ErrInvalidResponse) {
		value, err = client.AnalyzeArticle(ctx, request)
	}
	if err != nil {
		return ArticleAnalysis{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return value, nil
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
