package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/labstack/echo/v5"
)

func (s *Server) saveSettings(c *echo.Context) error {
	var value api.Settings
	if err := json.NewDecoder(c.Request().Body).Decode(&value); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	if err := validateSettings(value); err != nil {
		return writeBadRequest(c, err.Error())
	}
	resolved, err := s.settings.ResolveSecrets(c.Request().Context(), value)
	if err != nil {
		return writeAPIError(c, err)
	}
	if err := s.settings.Save(c.Request().Context(), resolved); err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, settingsResponse(resolved))
}

func (s *Server) updateLanguage(c *echo.Context) error {
	var request api.UpdateLanguageRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	if err := validateLanguage(request.Language); err != nil {
		return writeBadRequest(c, err.Error())
	}
	value, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	value.Language = request.Language
	if err := s.settings.Save(c.Request().Context(), value); err != nil {
		return writeAPIError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) listReports(c *echo.Context) error {
	values, err := s.reports.List(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, values)
}

func (s *Server) createReport(c *echo.Context) error {
	var request api.CreateReportRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	if request.Type != "weekly" && request.Type != "monthly" {
		return writeBadRequest(c, "type must be weekly or monthly")
	}
	start, startErr := parseDay(request.Start)
	end, endErr := parseDay(request.End)
	if startErr != nil || endErr != nil || start > end {
		return writeBadRequest(c, "invalid report period")
	}
	request.Start, request.End = start, end
	appSettings, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	value, err := s.reportService.Create(c.Request().Context(), request, report.CreateOptions{
		Language: appSettings.Language, TimezoneOffsetMinutes: appSettings.TimezoneOffsetMinutes,
	})
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusCreated, value)
}

func (s *Server) deleteReport(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return writeBadRequest(c, "invalid report id")
	}
	if err := s.reports.Delete(c.Request().Context(), id); err != nil {
		return writeAPIError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) testLLM(c *echo.Context) error {
	var err error
	if c.Request().ContentLength == 0 {
		err = s.summaries.TestConnection(c.Request().Context())
	} else {
		var value api.Settings
		if decodeErr := json.NewDecoder(c.Request().Body).Decode(&value); decodeErr != nil {
			return writeBadRequest(c, "invalid JSON body")
		}
		if validateErr := validateSettings(value); validateErr != nil {
			return writeBadRequest(c, validateErr.Error())
		}
		resolved, resolveErr := s.settings.ResolveSecrets(c.Request().Context(), value)
		if resolveErr != nil {
			return writeAPIError(c, resolveErr)
		}
		err = s.summaries.TestConnectionWithSettings(c.Request().Context(), resolved)
	}
	if err != nil {
		logServerError(c, http.StatusBadGateway, err)
		return c.JSON(http.StatusBadGateway, api.LLMTestResponse{
			Status:    "failed",
			Message:   bilingualMessage("연결에 실패했습니다. Base URL과 모델을 확인하세요.", "Connection failed. Check the Base URL and model."),
			MessageKO: "연결에 실패했습니다. Base URL과 모델을 확인하세요.",
			MessageEN: "Connection failed. Check the Base URL and model.",
		})
	}
	return c.JSON(http.StatusOK, api.LLMTestResponse{Status: "connected"})
}

func (s *Server) listLLMPresets(c *echo.Context) error {
	values, err := s.settings.Presets(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, llmPresetResponses(values))
}

func (s *Server) createLLMPreset(c *echo.Context) error {
	var request api.CreateLLMPresetRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	if err := validateLLMPreset(request); err != nil {
		return writeBadRequest(c, err.Error())
	}
	value, err := s.settings.CreatePreset(c.Request().Context(), request)
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusCreated, llmPresetResponse(value))
}

func (s *Server) deleteLLMPreset(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return writeBadRequest(c, "invalid LLM preset id")
	}
	if err := s.settings.DeletePreset(c.Request().Context(), id); err != nil {
		return writeAPIError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) updateLLMPreset(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return writeBadRequest(c, "invalid LLM preset id")
	}
	var request api.UpdateLLMPresetRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return writeBadRequest(c, "invalid JSON body")
	}
	if err := s.settings.UpdatePresetAPIKey(c.Request().Context(), id, request.APIKey); err != nil {
		return writeAPIError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
