package summary

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

var ErrNotConfigured = errors.New("LLM endpoint is not configured")
var ErrUnavailable = errors.New("LLM endpoint is unavailable")

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
	value, err := client.Generate(ctx, request)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return value, nil
}

func (s *Service) TestConnection(ctx context.Context) error {
	client, err := s.client(ctx)
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
