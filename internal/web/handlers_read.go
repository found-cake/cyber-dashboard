package web

import (
	"net/http"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/labstack/echo/v5"
)

func (s *Server) bootstrap(c *echo.Context) error {
	ctx := c.Request().Context()
	sources, err := s.feeds.Sources(ctx)
	if err != nil {
		return writeAPIError(c, err)
	}
	reports, err := s.reports.List(ctx)
	if err != nil {
		return writeAPIError(c, err)
	}
	appSettings, err := s.settings.Get(ctx)
	if err != nil {
		return writeAPIError(c, err)
	}
	days, err := s.feeds.CollectedDays(ctx)
	if err != nil {
		return writeAPIError(c, err)
	}
	presets, err := s.settings.Presets(ctx)
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, api.Bootstrap{
		Sources: sources, Reports: reports, Settings: settingsResponse(appSettings),
		LLMPresets: llmPresetResponses(presets), CollectedDays: days, Collection: s.collections.Active(),
	})
}

func (s *Server) dashboardData(c *echo.Context) error {
	value, err := s.dashboard.Dashboard(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, value)
}

func (s *Server) daily(c *echo.Context) error {
	day, err := parseDay(c.Param("day"))
	if err != nil {
		return writeBadRequest(c, err.Error())
	}
	value, err := s.feeds.Daily(c.Request().Context(), day)
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, value)
}
