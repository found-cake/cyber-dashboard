package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/collection"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
	"github.com/labstack/echo/v5"
)

func (s *Server) collect(c *echo.Context) error {
	var request api.CollectRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	appSettings, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	day, err := parseCollectableDay(request.Day, s.configuredTime(appSettings.TimezoneOffsetMinutes))
	if err != nil {
		return writeBadRequest(c, err.Error())
	}
	if strings.TrimSpace(appSettings.NVDAPIKey) == "" {
		return c.JSON(http.StatusPreconditionFailed, localizedError("nvd_key_required",
			"NVD API 키를 등록하세요", "Register an NVD API key before collecting articles"))
	}
	job, err := s.collections.Start(c.Request().Context(), day)
	if err != nil {
		if errors.Is(err, collection.ErrBusy) {
			return c.JSON(http.StatusConflict, localizedError("collection_busy", "다른 날짜를 수집 중입니다", "Another date is being collected"))
		}
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusAccepted, job)
}

func (s *Server) runCollection(ctx context.Context, day string) (api.CollectionResult, error) {
	appSettings, err := s.settings.Get(ctx)
	if err != nil {
		return api.CollectionResult{}, err
	}
	aiReady := s.summaries.TestConnection(ctx) == nil
	result, err := s.collector.Collect(ctx, day)
	if err != nil {
		return api.CollectionResult{}, err
	}
	if result.Collected > 0 && aiReady && s.articles != nil {
		if err := s.articles.EnrichDay(ctx, day, appSettings.Language); err != nil {
			slog.WarnContext(ctx, "article AI enrichment failed", slog.String("day", day), slog.String("response_body", err.Error()))
			result.Warnings = append(result.Warnings, bilingualMessage("기사 AI 분석에 실패했습니다", "Article AI analysis failed"))
		}
	}
	if result.Collected > 0 && s.vulnerabilities != nil {
		if err := s.vulnerabilities.EnrichDay(ctx, day); err != nil {
			slog.Warn("NVD enrichment failed", slog.String("day", day), slog.String("response_body", err.Error()))
			if errors.Is(err, vulnerability.ErrInvalidAPIKey) {
				result.Warnings = append(result.Warnings, bilingualMessage("NVD API 키가 유효하지 않습니다. 설정을 확인하세요", "The NVD API key is invalid. Check Settings"))
			} else {
				result.Warnings = append(result.Warnings, bilingualMessage("NVD 취약점 정보를 가져오지 못했습니다", "Failed to retrieve NVD vulnerability data"))
			}
		}
	}
	if !aiReady {
		result.Warnings = append(result.Warnings, bilingualMessage("AI API를 확인하세요", "Check the AI API"))
	} else if result.Collected > 0 {
		s.summaryForDay(ctx, day, &result.Warnings)
	}
	return result, nil
}

func (s *Server) collectionStatus(c *echo.Context) error {
	var job api.CollectionJob
	var err error
	if c.QueryParam("wait") == "1" {
		job, err = s.collections.Wait(c.Request().Context(), c.Param("id"))
	} else {
		job, err = s.collections.Get(c.Param("id"))
	}
	if errors.Is(err, collection.ErrNotFound) {
		return c.JSON(http.StatusNotFound, localizedError("collection_not_found", "수집 작업을 찾을 수 없습니다", "Collection job not found"))
	}
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, job)
}

func (s *Server) cancelCollection(c *echo.Context) error {
	if err := s.collections.Cancel(c.Param("id")); err != nil {
		if errors.Is(err, collection.ErrNotFound) {
			return c.JSON(http.StatusNotFound, localizedError("collection_not_found", "수집 작업을 찾을 수 없습니다", "Collection job not found"))
		}
		return writeAPIError(c, err)
	}
	job, err := s.collections.Wait(c.Request().Context(), c.Param("id"))
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, job)
}

func (s *Server) summaryForDay(ctx context.Context, day string, warnings *[]string) {
	daily, err := s.feeds.Daily(ctx, day)
	if err != nil {
		slog.WarnContext(ctx, "daily summary failed", slog.String("response_body", err.Error()))
		*warnings = append(*warnings, bilingualMessage("일간 요약 생성에 실패했습니다", "Daily summary generation failed"))
		return
	}
	appSettings, err := s.settings.Get(ctx)
	if err != nil {
		slog.WarnContext(ctx, "daily summary failed", slog.String("response_body", err.Error()))
		*warnings = append(*warnings, bilingualMessage("일간 요약 생성에 실패했습니다", "Daily summary generation failed"))
		return
	}
	facts := make([]string, 0, len(daily.Articles)+1)
	facts = append(facts, "day="+day)
	for _, article := range daily.Articles {
		facts = append(facts, fmt.Sprintf("title=%s\nsummary=%s\nseverity=%s\ncves=%s\nfull_article=%s",
			article.Title, article.Summary, article.Severity, strings.Join(article.CVEs, ","), article.Body))
	}
	value, err := s.summaries.Generate(ctx, summary.Request{
		Language: appSettings.Language,
		Kind:     "daily",
		Facts:    facts,
	})
	if err != nil {
		slog.WarnContext(ctx, "daily summary failed", slog.String("response_body", err.Error()))
		*warnings = append(*warnings, bilingualMessage("일간 요약 생성에 실패했습니다", "Daily summary generation failed"))
		return
	}
	if err := s.feeds.SaveDailySummary(ctx, day, value); err != nil {
		slog.WarnContext(ctx, "daily summary failed", slog.String("response_body", err.Error()))
		*warnings = append(*warnings, bilingualMessage("일간 요약 저장에 실패했습니다", "Daily summary save failed"))
	}
}
