package httpapi

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/collection"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
	feedstore "github.com/found-cake/cyber-dashboard/internal/feed/store"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
	"github.com/labstack/echo/v5"
)

type Dependencies struct {
	Assets          fs.FS
	Feeds           *feedstore.Repository
	Collector       *collector.Collector
	Dashboard       *dashboard.Repository
	Settings        *settings.Repository
	Reports         *report.Repository
	ReportService   *report.Service
	Summaries       *summary.Service
	Articles        ArticleEnricher
	Vulnerabilities VulnerabilityEnricher
	Now             func() time.Time
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
	now             func() time.Time
}

func NewServer(dependencies Dependencies) *Server {
	e := echo.New()
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
	}
	server.collections = collection.NewService(server.runCollection)
	if dependencies.Vulnerabilities != nil {
		server.cveRefreshes = vulnerability.NewRefreshJobs(dependencies.Vulnerabilities.RefreshAll)
	}
	e.GET("/healthz", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, api.HealthResponse{Status: "ok"})
	})
	e.GET("/api/bootstrap", server.bootstrap)
	e.GET("/api/dashboard", server.dashboardData)
	e.POST("/api/cves/refresh", server.refreshCVEs)
	e.GET("/api/cves/refresh/:id", server.cveRefreshStatus)
	e.GET("/api/daily/:day", server.daily)
	e.POST("/api/collect", server.collect)
	e.GET("/api/collect/:id", server.collectionStatus)
	e.DELETE("/api/collect/:id", server.cancelCollection)
	e.PUT("/api/settings", server.saveSettings)
	e.PATCH("/api/settings/language", server.updateLanguage)
	e.GET("/api/reports", server.listReports)
	e.POST("/api/reports", server.createReport)
	e.DELETE("/api/reports/:id", server.deleteReport)
	e.POST("/api/llm/test", server.testLLM)
	e.GET("/api/llm/presets", server.listLLMPresets)
	e.POST("/api/llm/presets", server.createLLMPreset)
	e.PUT("/api/llm/presets/:id", server.updateLLMPreset)
	e.DELETE("/api/llm/presets/:id", server.deleteLLMPreset)
	e.StaticFS("/", dependencies.Assets)
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.echo.ServeHTTP(writer, request)
}
