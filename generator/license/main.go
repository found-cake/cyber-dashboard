package main

//go:generate go run . -output static/legal

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	output := flag.String("output", "static/legal", "generated license directory")
	flag.Parse()

	if err := generate(context.Background(), *output); err != nil {
		fmt.Fprintf(os.Stderr, "generate license notices: %v\n", err)
		os.Exit(1)
	}
}

func generate(ctx context.Context, output string) error {
	root, err := findModuleRoot(ctx)
	if err != nil {
		return err
	}
	program, err := os.ReadFile(root + "/LICENSE")
	if err != nil {
		return fmt.Errorf("read project license: %w", err)
	}
	modules, err := discoverRuntimeModules(ctx, root)
	if err != nil {
		return err
	}
	standardLibrary, err := discoverStandardLibrary(ctx)
	if err != nil {
		return err
	}
	modules = append(modules, standardLibrary)
	assets, err := discoverBundledAssets(root)
	if err != nil {
		return err
	}
	outputDirectory := output
	if !filepath.IsAbs(outputDirectory) {
		outputDirectory = filepath.Join(root, outputDirectory)
	}
	return writeDocuments(outputDirectory, string(program), renderThirdPartyNotices(modules, assets))
}
