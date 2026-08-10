package httpapi

import (
	"net/http"
	"time"

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
		LLMPresets: llmPresetResponses(presets), CollectedDays: days, Collection: s.collections.Active(), CVERefresh: s.activeCVERefresh(),
	})
}

// configuredTime is the current time shifted onto the configured UTC offset, so its
// calendar date is the "today" the whole application agrees on. Formatting it with
// time.DateOnly yields that date because the returned time carries the UTC location.
func (s *Server) configuredTime(offsetMinutes int) time.Time {
	return s.now().UTC().Add(time.Duration(offsetMinutes) * time.Minute)
}

func (s *Server) dashboardData(c *echo.Context) error {
	appSettings, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	// The 30-day window is computed here rather than with SQLite's date('now'), which is
	// always UTC and would drift a day away from the configured timezone.
	since := s.configuredTime(appSettings.TimezoneOffsetMinutes).AddDate(0, 0, -29).Format(time.DateOnly)
	value, err := s.dashboard.Dashboard(c.Request().Context(), since)
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
