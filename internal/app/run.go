package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/auth"
	"github.com/found-cake/cyber-dashboard/internal/dashboard"
	"github.com/found-cake/cyber-dashboard/internal/database"
	feedbody "github.com/found-cake/cyber-dashboard/internal/feed/body"
	"github.com/found-cake/cyber-dashboard/internal/feed/collector"
	"github.com/found-cake/cyber-dashboard/internal/feed/enrichment"
	feedstore "github.com/found-cake/cyber-dashboard/internal/feed/store"
	"github.com/found-cake/cyber-dashboard/internal/report"
	"github.com/found-cake/cyber-dashboard/internal/settings"
	"github.com/found-cake/cyber-dashboard/internal/summary"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
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
		runErr = errors.Join(runErr, database.Close(db))
	}()
	settingsRepository, err := settings.NewRepository(db, databasePath+".key")
	if err != nil {
		return err
	}
	signingKey, err := auth.LoadOrCreateSigningKey(databasePath + ".jwt.key")
	if err != nil {
		return err
	}
	sessionStore, err := auth.OpenSessionStore(ctx, filepath.Join(dataDir, "dashboard.sessions.db"), nil)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, sessionStore.Close())
	}()
	authManager, err := auth.NewManager(db, sessionStore, signingKey)
	if err != nil {
		return err
	}
	initialPassword, generated, err := authManager.EnsurePassword(ctx)
	if err != nil {
		return err
	}
	if generated {
		if _, err := fmt.Fprintf(os.Stderr, "Initial dashboard password: %s\n", initialPassword); err != nil {
			return fmt.Errorf("print initial dashboard password: %w", err)
		}
		slog.Warn("generated initial dashboard password",
			slog.String("action", "log in and change it in Settings"))
	}
	trustedHosts, allowUntrustedHosts, err := trustedHostsFromEnvironment()
	if err != nil {
		return err
	}
	if allowUntrustedHosts {
		slog.Warn("trusted host protection disabled",
			slog.String("address", configuredAddress()),
			slog.String("environment_variable", "CYBER_DASHBOARD_TRUSTED_HOST=none"),
			slog.String("security_impact", "host allowlist and DNS rebinding protection are reduced"))
	} else if len(trustedHosts) == 0 {
		slog.Warn("dashboard is bound to all interfaces without a trusted host",
			slog.String("address", configuredAddress()),
			slog.String("set_environment_variable", "CYBER_DASHBOARD_TRUSTED_HOST"))
	}
	feedRepository := feedstore.NewRepository(db)
	reportRepository := report.NewRepository(db)
	summaryService := summary.NewService(settingsRepository)
	vulnerabilityService := vulnerability.NewService(feedRepository, settingsRepository, vulnerability.NewClient(nil, ""))
	browserBodyLoader := feedbody.NewChromiumBodyLoader(ctx)
	defer browserBodyLoader.Close()
	handler := web.NewServer(web.Dependencies{
		Assets:              assets,
		Feeds:               feedRepository,
		Collector:           collector.NewCollector(feedRepository, collector.NewHTTPFetcher(), feedbody.NewArticleBodyLoader(nil, browserBodyLoader)),
		Dashboard:           dashboard.NewRepository(db),
		Settings:            settingsRepository,
		Reports:             reportRepository,
		ReportService:       report.NewService(reportRepository, summaryService),
		Summaries:           summaryService,
		Articles:            enrichment.NewArticleEnrichmentService(feedRepository, summaryService),
		Vulnerabilities:     vulnerabilityService,
		TrustedHosts:        trustedHosts,
		AllowUntrustedHosts: allowUntrustedHosts,
		Auth:                authManager,
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
	address := configuredAddress()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("cyber dashboard listening", slog.String("address", "http://"+listener.Addr().String()))
		errCh <- server.Serve(listener)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		closeErr := listener.Close()
		if shutdownErr != nil {
			return fmt.Errorf("shutdown server: %w", shutdownErr)
		}
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			return fmt.Errorf("close server listener: %w", closeErr)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve dashboard: %w", err)
	}
}

func configuredAddress() string {
	if configured := os.Getenv("CYBER_DASHBOARD_ADDR"); configured != "" {
		return configured
	}
	return "127.0.0.1:13370"
}

func trustedHostsFromEnvironment() ([]string, bool, error) {
	addressHost, _, err := net.SplitHostPort(configuredAddress())
	if err != nil {
		return nil, false, fmt.Errorf("parse dashboard address: %w", err)
	}
	explicitTrustedHost := strings.TrimSpace(os.Getenv("CYBER_DASHBOARD_TRUSTED_HOST"))
	allowUntrustedHosts := explicitTrustedHost == "none"
	values := []string{addressHost}
	if !allowUntrustedHosts {
		values = append(values, explicitTrustedHost)
	}
	hosts := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		host, err := web.NormalizeTrustedHost(value)
		if err != nil {
			return nil, false, fmt.Errorf("parse trusted host %q: %w", value, err)
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			continue
		}
		hosts = append(hosts, host)
	}
	return hosts, allowUntrustedHosts, nil
}
