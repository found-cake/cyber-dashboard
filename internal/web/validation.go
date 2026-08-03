package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/feed"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/labstack/echo/v5"
)

func parseDay(raw string) (string, error) {
	parsed, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return "", fmt.Errorf("date must use YYYY-MM-DD")
	}
	return parsed.Format(time.DateOnly), nil
}

func parseCollectableDay(raw string, now time.Time) (string, error) {
	day, err := parseDay(raw)
	if err != nil {
		return "", err
	}
	parsed, _ := time.Parse(time.DateOnly, day)
	today, _ := time.Parse(time.DateOnly, now.Format(time.DateOnly))
	if parsed.After(today) {
		return "", fmt.Errorf("future dates cannot be collected")
	}
	if parsed.Before(today.AddDate(0, 0, -9)) {
		return "", fmt.Errorf("date is outside the 10-day feed retention window")
	}
	return day, nil
}

func validateSettings(value api.Settings) error {
	if value.Language != "ko" && value.Language != "en" {
		return fmt.Errorf("language must be ko or en")
	}
	if value.Theme != "dark" && value.Theme != "light" {
		return fmt.Errorf("theme must be dark or light")
	}
	parsed, err := url.ParseRequestURI(value.LLMBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("LLM base URL must be an absolute HTTP URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("LLM base URL must use HTTP or HTTPS")
	}
	if strings.TrimSpace(value.LLMModel) == "" || value.LLMTimeout < 1 || value.LLMTimeout > 600 {
		return fmt.Errorf("model and timeout are required")
	}
	return nil
}

func validateLLMPreset(value api.CreateLLMPresetRequest) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("LLM base URL must be an absolute HTTP URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("LLM base URL must use HTTP or HTTPS")
	}
	if strings.TrimSpace(value.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

func writeBadRequest(c *echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, api.ErrorResponse{Code: "bad_request", Message: message})
}

func writeAPIError(c *echo.Context, err error) error {
	if errors.Is(err, feed.ErrNotFound) || errors.Is(err, report.ErrNotFound) {
		return c.JSON(http.StatusNotFound, api.ErrorResponse{Code: "not_found", Message: err.Error()})
	}
	if errors.Is(err, settings.ErrPresetNotFound) {
		return c.JSON(http.StatusNotFound, api.ErrorResponse{Code: "not_found", Message: err.Error()})
	}
	if errors.Is(err, settings.ErrBuiltinPreset) {
		return c.JSON(http.StatusForbidden, api.ErrorResponse{Code: "builtin_preset", Message: err.Error()})
	}
	if errors.Is(err, settings.ErrDuplicatePreset) {
		return c.JSON(http.StatusConflict, api.ErrorResponse{Code: "duplicate_preset", Message: err.Error()})
	}
	if errors.Is(err, summary.ErrNotConfigured) || errors.Is(err, summary.ErrUnavailable) {
		return c.JSON(http.StatusBadGateway, api.ErrorResponse{Code: "llm_unavailable", Message: err.Error()})
	}
	slog.Error("request failed", slog.Any("error", err))
	return c.JSON(http.StatusInternalServerError, api.ErrorResponse{Code: "internal", Message: "internal error"})
}
