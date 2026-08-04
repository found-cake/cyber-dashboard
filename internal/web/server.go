package web

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/collection"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/feed"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/labstack/echo/v5"
)

type Dependencies struct {
	Assets          fs.FS
	Feeds           *feed.Repository
	Collector       *feed.Collector
	Dashboard       *dashboard.Repository
	Settings        *settings.Repository
	Reports         *report.Repository
	ReportService   *report.Service
	Summaries       *summary.Service
	Articles        ArticleEnricher
	Vulnerabilities VulnerabilityEnricher
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
	feeds           *feed.Repository
	collector       *feed.Collector
	dashboard       *dashboard.Repository
	settings        *settings.Repository
	reports         *report.Repository
	reportService   *report.Service
	summaries       *summary.Service
	articles        ArticleEnricher
	vulnerabilities VulnerabilityEnricher
	collections     *collection.Service
}

func NewServer(dependencies Dependencies) *Server {
	e := echo.New()
	server := &Server{
		echo: e, feeds: dependencies.Feeds, collector: dependencies.Collector,
		dashboard: dependencies.Dashboard, settings: dependencies.Settings,
		reports: dependencies.Reports, reportService: dependencies.ReportService,
		summaries:       dependencies.Summaries,
		articles:        dependencies.Articles,
		vulnerabilities: dependencies.Vulnerabilities,
	}
	server.collections = collection.NewService(server.runCollection)
	e.GET("/healthz", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, api.HealthResponse{Status: "ok"})
	})
	e.GET("/api/bootstrap", server.bootstrap)
	e.GET("/api/dashboard", server.dashboardData)
	e.POST("/api/cves/refresh", server.refreshCVEs)
	e.GET("/api/daily/:day", server.daily)
	e.POST("/api/collect", server.collect)
	e.GET("/api/collect/:id", server.collectionStatus)
	e.DELETE("/api/collect/:id", server.cancelCollection)
	e.PATCH("/api/sources/:id", server.toggleSource)
	e.PUT("/api/settings", server.saveSettings)
	e.PATCH("/api/settings/language", server.updateLanguage)
	e.GET("/api/reports", server.listReports)
	e.POST("/api/reports", server.createReport)
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
