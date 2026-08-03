package cyberdashboard

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var embeddedFrontend embed.FS

// EmbeddedFrontend returns the frontend bundled into cmd/cyber-dashboard-full.
func EmbeddedFrontend() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "static")
}
