package httpapi

import (
	"context"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/auth"
	"github.com/found-cake/cyber-dashboard/internal/collection"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
	feedstore "github.com/found-cake/cyber-dashboard/internal/feed/store"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// Only JSON endpoints are compressed; the static frontend is served as-is.
const (
	apiPathPrefix              = "/api/"
	gzipMinimumLength          = 1024
	staticAssetCacheControl    = "public, max-age=3600"
	staticDocumentCacheControl = "no-cache"
)

type Dependencies struct {
	Assets              fs.FS
	Feeds               *feedstore.Repository
	Collector           *collector.Collector
	Dashboard           *dashboard.Repository
	Settings            *settings.Repository
	Reports             *report.Repository
	ReportService       *report.Service
	Summaries           *summary.Service
	Articles            ArticleEnricher
	Vulnerabilities     VulnerabilityEnricher
	Now                 func() time.Time
	TrustedHosts        []string
	AllowUntrustedHosts bool
	Auth                *auth.Manager
}

type ArticleEnricher interface {
	EnrichDay(ctx context.Context, day, language string) error
}

type VulnerabilityEnricher interface {
	EnrichDay(ctx context.Context, day string) error
	RefreshAll(ctx context.Context) (api.CVERefreshResult, error)
}

type Server struct {
	echo            *echo.Echo
	feeds           *feedstore.Repository
	collector       *collector.Collector
	dashboard       *dashboard.Repository
	settings        *settings.Repository
	reports         *report.Repository
	reportService   *report.Service
	summaries       *summary.Service
	articles        ArticleEnricher
	vulnerabilities VulnerabilityEnricher
	collections     *collection.Service
	cveRefreshes    *vulnerability.RefreshJobs
	auth            *auth.Manager
	loginLimiter    *loginLimiter
	now             func() time.Time
}

func NewServer(dependencies Dependencies) *Server {
	e := echo.New()
	hostGuard := newHostGuard(dependencies.TrustedHosts, dependencies.AllowUntrustedHosts)
	e.Pre(hostGuard.middleware)
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c *echo.Context) bool {
			if !strings.HasPrefix(c.Request().URL.Path, apiPathPrefix) {
				return true
			}
			if acceptsGzip(c.Request().Header.Get(echo.HeaderAcceptEncoding)) {
				c.Request().Header.Set(echo.HeaderAcceptEncoding, "gzip")
				return false
			}
			c.Response().Header().Add(echo.HeaderVary, echo.HeaderAcceptEncoding)
			return true
		},
		MinLength: gzipMinimumLength,
	}))
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	server := &Server{
		echo: e, feeds: dependencies.Feeds, collector: dependencies.Collector,
		dashboard: dependencies.Dashboard, settings: dependencies.Settings,
		reports: dependencies.Reports, reportService: dependencies.ReportService,
		summaries:       dependencies.Summaries,
		articles:        dependencies.Articles,
		vulnerabilities: dependencies.Vulnerabilities,
		now:             now,
		auth:            dependencies.Auth,
	}
	server.collections = collection.NewService(server.runCollection)
	server.loginLimiter = newLoginLimiter(now)
	if dependencies.Vulnerabilities != nil {
		server.cveRefreshes = vulnerability.NewRefreshJobs(dependencies.Vulnerabilities.RefreshAll)
	}
	e.GET("/healthz", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, api.HealthResponse{Status: "ok"})
	})
	e.GET("/api/bootstrap", server.bootstrap)
	e.GET("/api/dashboard", server.dashboardData)
	e.GET("/api/cves", server.listCVEs)
	e.GET("/api/cves/refresh/:id", server.cveRefreshStatus)
	e.GET("/api/daily/:day", server.daily)
	e.GET("/api/collect/:id", server.collectionStatus)
	e.GET("/api/reports", server.listReports)
	e.GET("/api/reports/:id", server.getReport)
	e.POST("/api/auth/login", server.login)
	e.POST("/api/auth/refresh", server.refreshSession)
	e.POST("/api/auth/logout", server.logout)
	e.POST("/api/cves/refresh", server.refreshCVEs, server.requireAuth)
	e.POST("/api/collect", server.collect, server.requireAuth)
	e.DELETE("/api/collect/:id", server.cancelCollection, server.requireAuth)
	e.PUT("/api/settings", server.saveSettings, server.requireAuth)
	e.PATCH("/api/settings/language", server.updateLanguage, server.requireAuth)
	e.POST("/api/reports", server.createReport, server.requireAuth)
	e.DELETE("/api/reports/:id", server.deleteReport, server.requireAuth)
	e.POST("/api/llm/test", server.testLLM, server.requireAuth)
	e.GET("/api/llm/presets", server.listLLMPresets, server.requireAuth)
	e.POST("/api/llm/presets", server.createLLMPreset, server.requireAuth)
	e.PUT("/api/llm/presets/:id", server.updateLLMPreset, server.requireAuth)
	e.DELETE("/api/llm/presets/:id", server.deleteLLMPreset, server.requireAuth)
	e.PUT("/api/auth/password", server.changePassword, server.requireAuth)
	e.StaticFS("/", dependencies.Assets, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			requestPath := c.Request().URL.Path
			cacheControl := staticAssetCacheControl
			if requestPath == "/app.js" || strings.HasSuffix(requestPath, "/") || strings.HasSuffix(requestPath, ".html") {
				cacheControl = staticDocumentCacheControl
			}
			c.Response().Header().Set(echo.HeaderCacheControl, cacheControl)
			if err := next(c); err != nil {
				c.Response().Header().Del(echo.HeaderCacheControl)
				return err
			}
			return nil
		}
	})
	return server
}

func acceptsGzip(header string) bool {
	wildcardAccepted := false
	gzipSpecified := false
	for value := range strings.SplitSeq(header, ",") {
		candidate := strings.TrimSpace(value)
		coding, parameterText, hasParameters := strings.Cut(candidate, ";")
		isWildcard := strings.TrimSpace(coding) == "*"
		if isWildcard {
			candidate = "wildcard"
			if hasParameters {
				candidate += ";" + parameterText
			}
		}
		encoding, parameters, err := mime.ParseMediaType(candidate)
		if err != nil || !strings.EqualFold(encoding, "gzip") && !isWildcard {
			continue
		}
		isGzip := strings.EqualFold(encoding, "gzip")
		if isGzip {
			gzipSpecified = true
		}
		quality := 1.0
		qualityValue, exists := parameters["q"]
		if exists {
			parsed, parseErr := strconv.ParseFloat(qualityValue, 64)
			if parseErr != nil || parsed < 0 || parsed > 1 {
				continue
			}
			quality = parsed
		}
		if isGzip && quality > 0 {
			return true
		}
		if !isGzip && quality > 0 {
			wildcardAccepted = true
		}
	}
	return !gzipSpecified && wildcardAccepted
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.echo.ServeHTTP(writer, request)
}
