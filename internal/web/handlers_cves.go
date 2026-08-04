package web

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

func (s *Server) refreshCVEs(c *echo.Context) error {
	if s.vulnerabilities == nil {
		return c.JSON(http.StatusServiceUnavailable, localizedError("nvd_unavailable",
			"NVD 갱신 기능을 사용할 수 없습니다", "NVD refresh is unavailable"))
	}
	result, err := s.vulnerabilities.RefreshAll(c.Request().Context())
	if err != nil {
		return writeAPIError(c, err)
	}
	if len(result.Warnings) > 0 {
		slog.WarnContext(c.Request().Context(), "CVE refresh completed with warnings",
			slog.String("response_body", strings.Join(result.Warnings, "\n")))
	}
	return c.JSON(http.StatusOK, result)
}
