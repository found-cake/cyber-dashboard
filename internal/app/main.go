package app

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Main runs the shared process lifecycle for dashboard commands.
func Main(assets fs.FS) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := Run(ctx, assets); err != nil {
		slog.Error("cyber dashboard stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
