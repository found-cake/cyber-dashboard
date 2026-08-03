package main

import (
	"log/slog"
	"os"

	cyberdashboard "github.com/found-cake/cyber-dashboard"
	"github.com/found-cake/cyber-dashboard/internal/app"
)

func main() {
	assets, err := cyberdashboard.EmbeddedFrontend()
	if err != nil {
		slog.Error("open embedded frontend", slog.Any("error", err))
		os.Exit(1)
	}
	app.Main(assets)
}
