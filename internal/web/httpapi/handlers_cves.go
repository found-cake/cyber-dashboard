package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
	"github.com/labstack/echo/v5"
)

const (
	cveRevisionHeader = "X-CVE-Revision"
	cveCursorHeader   = "X-CVE-Cursor"
)

func (s *Server) listCVEs(c *echo.Context) error {
	sort := dashboard.CVESortScore
	if value := c.QueryParam("sort"); value != "" {
		parsed, ok := dashboard.ParseCVESort(value)
		if !ok {
			return writeBadRequest(c, "sort must be score, cvss, mentions, or firstSeen")
		}
		sort = parsed
	}
	if c.QueryParam("offset") != "" {
		return writeBadRequest(c, "offset is not supported")
	}
	cursor := c.QueryParam("cursor")
	var expectedRevision *uint64
	if value := c.QueryParam("revision"); value != "" {
		parsed, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil {
			return writeBadRequest(c, "revision must be an unsigned integer")
		}
		expectedRevision = &parsed
	}
	if cursor != "" && expectedRevision == nil {
		return writeBadRequest(c, "revision is required after the first CVE page")
	}
	page, err := s.dashboard.CVEInsights(c.Request().Context(), dashboard.CVEPageRequest{
		Sort: sort, Cursor: cursor, ExpectedRevision: expectedRevision,
	})
	if errors.Is(err, dashboard.ErrCVEPageStale) {
		return c.JSON(http.StatusConflict, localizedError("cve_page_stale",
			"CVE 정렬이 변경되어 처음부터 다시 불러옵니다", "The CVE ranking changed; reload from the first page"))
	}
	if errors.Is(err, dashboard.ErrCVECursorInvalid) {
		return writeBadRequest(c, "invalid CVE cursor")
	}
	if err != nil {
		return writeAPIError(c, err)
	}
	c.Response().Header().Set(cveRevisionHeader, strconv.FormatUint(page.Revision, 10))
	if page.NextCursor != "" {
		c.Response().Header().Set(cveCursorHeader, page.NextCursor)
	}
	return c.JSON(http.StatusOK, page.Values)
}

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
