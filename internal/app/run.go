package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/feed"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/web"
)

func Run(ctx context.Context, assets fs.FS) (runErr error) {
	if _, err := fs.Stat(assets, "index.html"); err != nil {
		return fmt.Errorf("open static assets: %w", err)
	}
	dataDir, err := resolveDataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	databasePath := filepath.Join(dataDir, "dashboard.db")
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, db.Close())
	}()
	settingsRepository, err := settings.NewRepository(db, databasePath+".key")
	if err != nil {
		return err
	}
	feedRepository := feed.NewRepository(db)
	reportRepository := report.NewRepository(db)
	summaryService := summary.NewService(settingsRepository)
	handler := web.NewServer(web.Dependencies{
		Assets:        assets,
		Feeds:         feedRepository,
		Collector:     feed.NewCollector(feedRepository, feed.NewHTTPFetcher()),
		Dashboard:     dashboard.NewRepository(db),
		Settings:      settingsRepository,
		Reports:       reportRepository,
		ReportService: report.NewService(reportRepository, summaryService),
		Summaries:     summaryService,
	})
	return serve(ctx, handler)
}

func resolveDataDir() (string, error) {
	if configured := os.Getenv("CYBER_DASHBOARD_DATA_DIR"); configured != "" {
		return configured, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config directory: %w", err)
	}
	return filepath.Join(configDir, "cyber-dashboard"), nil
}

func serve(ctx context.Context, handler http.Handler) error {
	address := "127.0.0.1:8080"
	if configured := os.Getenv("CYBER_DASHBOARD_ADDR"); configured != "" {
		address = configured
	}
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("cyber dashboard listening", slog.String("address", "http://"+address))
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve dashboard: %w", err)
	}
}
