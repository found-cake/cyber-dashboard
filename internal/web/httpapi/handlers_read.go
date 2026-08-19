package httpapi

import (
	"net/http"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/labstack/echo/v5"
)

func (s *Server) bootstrap(c *echo.Context) error {
	ctx := c.Request().Context()
	authenticated, err := s.authenticated(c)
	if err != nil {
		return writeAPIError(c, err)
	}
	sources := []api.Source{}
	presets := []api.LLMPresetResponse{}
	reports, err := s.reports.Summaries(ctx)
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
	settingsValue := api.SettingsResponse{
		Language: appSettings.Language, Accent: appSettings.Accent,
		TimezoneOffsetMinutes: appSettings.TimezoneOffsetMinutes,
	}
	if authenticated {
		sources, err = s.feeds.Sources(ctx)
		if err != nil {
			return writeAPIError(c, err)
		}
		values, err := s.settings.Presets(ctx)
		if err != nil {
			return writeAPIError(c, err)
		}
		presets = llmPresetResponses(values)
		settingsValue = settingsResponse(appSettings)
	}
	return c.JSON(http.StatusOK, api.Bootstrap{
		Auth:    api.AuthState{Enabled: s.auth != nil, Authenticated: authenticated},
		Sources: sources, Reports: reports, Settings: settingsValue,
		LLMPresets: presets, CollectedDays: days, Collection: s.collections.Active(), CVERefresh: s.activeCVERefresh(),
	})
}

// configuredTime shifts UTC so DateOnly formatting yields the application's configured today.
func (s *Server) configuredTime(offsetMinutes int) time.Time {
	return s.now().UTC().Add(time.Duration(offsetMinutes) * time.Minute)
}

// dashboardWindows are the offered ranges; 30 and 90 bucket to the same ten points on purpose.
var dashboardWindows = map[string]dashboard.Window{
	"":   {Days: 30, Bucket: 3},
	"7":  {Days: 7, Bucket: 1},
	"30": {Days: 30, Bucket: 3},
	"90": {Days: 90, Bucket: 9},
}

func (s *Server) dashboardData(c *echo.Context) error {
	window, allowed := dashboardWindows[c.QueryParam("days")]
	if !allowed {
		return writeBadRequest(c, "days must be 7, 30, or 90")
	}
	appSettings, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	// Compute the window here because SQLite date('now') always uses UTC.
	window.Since = s.configuredTime(appSettings.TimezoneOffsetMinutes).AddDate(0, 0, -(window.Days - 1)).Format(time.DateOnly)
	value, err := s.dashboard.Dashboard(c.Request().Context(), window, c.QueryParam("hide_none") == "1")
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
