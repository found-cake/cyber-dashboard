package main

import (
	"os"

	"github.com/found-cake/cyber-dashboard/internal/app"
)

const staticDirectoryEnvironment = "CYBER_DASHBOARD_STATIC_DIR"

func main() {
	staticDirectory := os.Getenv(staticDirectoryEnvironment)
	if staticDirectory == "" {
		staticDirectory = "static"
	}
	app.Main(os.DirFS(staticDirectory))
}
