package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/found-cake/cyber-dashboard/api"
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
	if err := s.settings.Save(c.Request().Context(), value); err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, value)
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
	value, err := s.reportService.Create(c.Request().Context(), request, appSettings.Language)
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusCreated, value)
}

func (s *Server) testLLM(c *echo.Context) error {
	if err := s.summaries.TestConnection(c.Request().Context()); err != nil {
		return c.JSON(http.StatusBadGateway, api.LLMTestResponse{
			Status: "failed", Message: "연결에 실패했습니다. Base URL과 모델을 확인하세요.",
		})
	}
	return c.JSON(http.StatusOK, api.LLMTestResponse{Status: "connected"})
}

func (s *Server) listLLMPresets(c *echo.Context) error {
	values, err := s.settings.Presets(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	return c.JSON(http.StatusOK, values)
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
	return c.JSON(http.StatusCreated, value)
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
