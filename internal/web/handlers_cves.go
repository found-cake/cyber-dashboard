package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
	"github.com/labstack/echo/v5"
)

func (s *Server) refreshCVEs(c *echo.Context) error {
	if s.cveRefreshes == nil {
		return c.JSON(http.StatusServiceUnavailable, localizedError("nvd_unavailable",
			"NVD 갱신 기능을 사용할 수 없습니다", "NVD refresh is unavailable"))
	}
	if active := s.cveRefreshes.Active(); active != nil {
		return writeCVERefreshStarted(c, *active)
	}
	appSettings, err := s.settings.Get(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	if strings.TrimSpace(appSettings.NVDAPIKey) == "" {
		return c.JSON(http.StatusPreconditionFailed, localizedError("nvd_key_required",
			"NVD API 키를 등록하세요", "Register an NVD API key"))
	}
	return writeCVERefreshStarted(c, s.cveRefreshes.Start(c.Request().Context()))
}

func writeCVERefreshStarted(c *echo.Context, job api.CVERefreshJob) error {
	c.Response().Header().Set("Location", "/api/cves/refresh/"+job.ID)
	return c.JSON(http.StatusAccepted, job)
}

func (s *Server) activeCVERefresh() *api.CVERefreshJob {
	if s.cveRefreshes == nil {
		return nil
	}
	return s.cveRefreshes.Active()
}

func (s *Server) cveRefreshStatus(c *echo.Context) error {
	if s.cveRefreshes == nil {
		return c.JSON(http.StatusServiceUnavailable, localizedError("nvd_unavailable",
			"NVD 갱신 기능을 사용할 수 없습니다", "NVD refresh is unavailable"))
	}
	var job api.CVERefreshJob
	var err error
	if c.QueryParam("wait") == "1" {
		job, err = s.cveRefreshes.Wait(c.Request().Context(), c.Param("id"))
	} else {
		job, err = s.cveRefreshes.Get(c.Param("id"))
	}
	if errors.Is(err, vulnerability.ErrRefreshJobNotFound) {
		return c.JSON(http.StatusNotFound, localizedError("cve_refresh_not_found",
			"CVE 갱신 작업을 찾을 수 없습니다", "CVE refresh job not found"))
	}
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, job)
}
