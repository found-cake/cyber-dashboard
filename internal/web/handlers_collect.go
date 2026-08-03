package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/labstack/echo/v5"
)

func (s *Server) collect(c *echo.Context) error {
	var request api.CollectRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	day, err := parseCollectableDay(request.Day, time.Now())
	if err != nil {
		return writeBadRequest(c, err.Error())
	}
	result, err := s.collector.Collect(c.Request().Context(), day)
	if err != nil {
		return writeAPIError(c, err)
	}
	if result.Collected > 0 {
		s.summaryForDay(c.Request().Context(), day, &result.Warnings)
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) summaryForDay(ctx context.Context, day string, warnings *[]string) {
	daily, err := s.feeds.Daily(ctx, day)
	if err != nil {
		*warnings = append(*warnings, "daily summary: "+err.Error())
		return
	}
	appSettings, err := s.settings.Get(ctx)
	if err != nil {
		*warnings = append(*warnings, "daily summary: "+err.Error())
		return
	}
	facts := make([]string, 0, len(daily.Articles)+1)
	facts = append(facts, "day="+day)
	for _, article := range daily.Articles {
		facts = append(facts, fmt.Sprintf("%s | %s | severity=%s | cves=%s",
			article.Title, article.Summary, article.Severity, strings.Join(article.CVEs, ",")))
	}
	value, err := s.summaries.Generate(ctx, summary.Request{
		Language: appSettings.Language,
		Kind:     "daily",
		Facts:    facts,
	})
	if err != nil {
		*warnings = append(*warnings, "daily summary: "+err.Error())
		return
	}
	if err := s.feeds.SaveDailySummary(ctx, day, value); err != nil {
		*warnings = append(*warnings, "daily summary: "+err.Error())
	}
}

func (s *Server) toggleSource(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return writeBadRequest(c, "invalid source id")
	}
	var request api.ToggleSourceRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	if err := s.feeds.SetSourceEnabled(c.Request().Context(), id, request.Enabled); err != nil {
		return writeAPIError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
