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
	if value.TimezoneOffsetMinutes < -12*60 || value.TimezoneOffsetMinutes > 14*60 || value.TimezoneOffsetMinutes%15 != 0 {
		return fmt.Errorf("timezone offset must be between UTC-12 and UTC+14 in 15-minute increments")
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
	korean := map[string]string{
		"invalid JSON body":                                "JSON 본문이 올바르지 않습니다",
		"date must use YYYY-MM-DD":                         "날짜는 YYYY-MM-DD 형식이어야 합니다",
		"future dates cannot be collected":                 "미래 날짜는 수집할 수 없습니다",
		"date is outside the 10-day feed retention window": "피드 보존 기간 10일을 벗어난 날짜입니다",
		"type must be weekly or monthly":                   "보고서 유형은 weekly 또는 monthly여야 합니다",
		"invalid report period":                            "보고서 기간이 올바르지 않습니다",
		"invalid LLM preset id":                            "LLM 프리셋 ID가 올바르지 않습니다",
		"invalid source id":                                "뉴스 출처 ID가 올바르지 않습니다",
		"language must be ko or en":                        "언어는 ko 또는 en이어야 합니다",
		"theme must be dark or light":                      "테마는 dark 또는 light여야 합니다",
		"LLM base URL must be an absolute HTTP URL":        "LLM Base URL은 절대 HTTP URL이어야 합니다",
		"LLM base URL must use HTTP or HTTPS":              "LLM Base URL은 HTTP 또는 HTTPS를 사용해야 합니다",
		"model and timeout are required":                   "모델과 타임아웃이 필요합니다",
		"timezone offset must be between UTC-12 and UTC+14 in 15-minute increments": "시간대는 UTC-12부터 UTC+14까지 15분 단위여야 합니다",
		"model is required": "모델이 필요합니다",
	}
	messageKO := korean[message]
	if messageKO == "" {
		messageKO = "요청이 올바르지 않습니다"
	}
	return c.JSON(http.StatusBadRequest, localizedError("bad_request", messageKO, message))
}

func writeAPIError(c *echo.Context, err error) error {
	if errors.Is(err, feed.ErrNotFound) || errors.Is(err, report.ErrNotFound) {
		return c.JSON(http.StatusNotFound, localizedError("not_found", "요청한 항목을 찾을 수 없습니다", "The requested resource was not found"))
	}
	if errors.Is(err, settings.ErrPresetNotFound) {
		return c.JSON(http.StatusNotFound, localizedError("not_found", "LLM 프리셋을 찾을 수 없습니다", "The LLM preset was not found"))
	}
	if errors.Is(err, settings.ErrBuiltinPreset) {
		return c.JSON(http.StatusForbidden, localizedError("builtin_preset", "기본 프리셋은 삭제할 수 없습니다", "Built-in presets cannot be deleted"))
	}
	if errors.Is(err, settings.ErrDuplicatePreset) {
		return c.JSON(http.StatusConflict, localizedError("duplicate_preset", "같은 서버와 모델의 프리셋이 이미 있습니다", "A preset for the same server and model already exists"))
	}
	if errors.Is(err, summary.ErrNotConfigured) || errors.Is(err, summary.ErrUnavailable) {
		logServerError(c, http.StatusBadGateway, err)
		return c.JSON(http.StatusBadGateway, localizedError("llm_unavailable", "AI API 설정과 연결 상태를 확인하세요", "Check the AI API configuration and connection"))
	}
	logServerError(c, http.StatusInternalServerError, err)
	return c.JSON(http.StatusInternalServerError, localizedError("internal", "서버 오류가 발생했습니다", "An internal server error occurred"))
}

func localizedError(code, korean, english string) api.ErrorResponse {
	return api.ErrorResponse{
		Code: code, Message: bilingualMessage(korean, english), MessageKO: korean, MessageEN: english,
	}
}

func bilingualMessage(korean, english string) string {
	return korean + " / " + english
}

func logServerError(c *echo.Context, status int, err error) {
	slog.ErrorContext(c.Request().Context(), "server request failed",
		slog.String("method", c.Request().Method), slog.String("path", c.Request().URL.Path),
		slog.Int("status", status), slog.String("response_body", err.Error()))
}
