package httpapi

import (
	"net/http"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
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

// dashboardWindows are the aggregation ranges the dashboard offers; an absent value keeps
// the original 30-day default.
var dashboardWindows = map[string]int{"": 30, "7": 7, "30": 30}

func (s *Server) dashboardData(c *echo.Context) error {
	days, allowed := dashboardWindows[c.QueryParam("days")]
	if !allowed {
		return writeBadRequest(c, "days must be 7 or 30")
	}
	appSettings, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	// Compute the window here because SQLite date('now') always uses UTC.
	since := s.configuredTime(appSettings.TimezoneOffsetMinutes).AddDate(0, 0, -(days - 1)).Format(time.DateOnly)
	value, err := s.dashboard.Dashboard(c.Request().Context(), since, c.QueryParam("hide_none") == "1")
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
