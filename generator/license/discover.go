package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type listedModule struct {
	Path    string
	Version string
	Dir     string
	Main    bool
	Replace *listedModule
}

type listedPackage struct {
	Standard bool
	Module   *listedModule
}

type licenseDocument struct {
	Name string
	Text string
}

type moduleNotice struct {
	Path     string
	Version  string
	Licenses []licenseDocument
}

type assetNotice struct {
	Path   string
	Notice string
}

type buildTarget struct {
	goos   string
	goarch string
}

var releaseTargets = []buildTarget{
	{goos: "linux", goarch: "amd64"},
	{goos: "linux", goarch: "arm64"},
	{goos: "darwin", goarch: "amd64"},
	{goos: "darwin", goarch: "arm64"},
	{goos: "windows", goarch: "amd64"},
	{goos: "windows", goarch: "arm64"},
}

func findModuleRoot(ctx context.Context) (string, error) {
	command := exec.CommandContext(ctx, "go", "env", "GOMOD")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("locate go.mod: %w", err)
	}
	moduleFile := strings.TrimSpace(string(output))
	if moduleFile == "" || moduleFile == os.DevNull {
		return "", fmt.Errorf("locate go.mod: command returned %q", moduleFile)
	}
	return filepath.Dir(moduleFile), nil
}

func discoverRuntimeModules(ctx context.Context, root string) ([]moduleNotice, error) {
	modules := make(map[string]listedModule)
	for _, target := range releaseTargets {
		listed, err := listRuntimeModules(ctx, root, target)
		if err != nil {
			return nil, err
		}
		for _, module := range listed {
			modules[module.Path] = module
		}
	}

	result := make([]moduleNotice, 0, len(modules))
	for _, module := range modules {
		licenses, err := readLicenseDocuments(module.Dir)
		if err != nil {
			return nil, fmt.Errorf("read licenses for %s: %w", module.Path, err)
		}
		result = append(result, moduleNotice{Path: module.Path, Version: module.Version, Licenses: licenses})
	}
	return result, nil
}

func listRuntimeModules(ctx context.Context, root string, target buildTarget) ([]listedModule, error) {
	command := exec.CommandContext(ctx, "go", "list", "-deps", "-json", "./cmd/cyber-dashboard-full", "./cmd/cyber-dashboard-server-only")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.goos, "GOARCH="+target.goarch)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list runtime dependencies for %s/%s: %w", target.goos, target.goarch, err)
	}

	result := make([]listedModule, 0, 32)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decode go list output for %s/%s: %w", target.goos, target.goarch, err)
		}
		if pkg.Standard || pkg.Module == nil || pkg.Module.Main {
			continue
		}
		module := *pkg.Module
		if module.Replace != nil {
			module.Dir = module.Replace.Dir
		}
		result = append(result, module)
	}
	return result, nil
}

func discoverStandardLibrary(ctx context.Context) (moduleNotice, error) {
	command := exec.CommandContext(ctx, "go", "env", "GOROOT")
	output, err := command.Output()
	if err != nil {
		return moduleNotice{}, fmt.Errorf("locate GOROOT: %w", err)
	}
	license, err := readStandardLibraryLicense(strings.TrimSpace(string(output)))
	if err != nil {
		return moduleNotice{}, fmt.Errorf("read Go license: %w", err)
	}
	return moduleNotice{
		Path: "Go runtime and standard library",
		Licenses: []licenseDocument{{
			Name: "LICENSE",
			Text: license,
		}},
	}, nil
}

func readStandardLibraryLicense(goRoot string) (string, error) {
	license, err := os.ReadFile(filepath.Join(goRoot, "LICENSE"))
	if err == nil {
		return string(license), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return goStandardLibraryLicense, nil
	}
	return "", err
}

func readLicenseDocuments(directory string) ([]licenseDocument, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	result := make([]licenseDocument, 0, 2)
	for _, entry := range entries {
		if entry.IsDir() || !isLicenseFilename(entry.Name()) {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, licenseDocument{Name: entry.Name(), Text: string(contents)})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no top-level license, copying, or notice file in %s", directory)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result, nil
}

func isLicenseFilename(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "COPYING") || strings.HasPrefix(upper, "NOTICE")
}

func discoverBundledAssets(root string) ([]assetNotice, error) {
	staticRoot := filepath.Join(root, "static")
	result := make([]assetNotice, 0, 1)
	err := filepath.WalkDir(staticRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (filepath.Ext(path) != ".js" && filepath.Ext(path) != ".css") {
			return nil
		}
		notice, err := readAssetLicenseHeader(path)
		if err != nil {
			return err
		}
		if notice == "" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result = append(result, assetNotice{Path: filepath.ToSlash(relative), Notice: notice})
		return nil
	})
	return result, err
}

func readAssetLicenseHeader(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		start := strings.Index(line, "/*!")
		end := strings.Index(line, "*/")
		if start < 0 || end < start || !strings.Contains(strings.ToLower(line[start:end]), "license") {
			continue
		}
		return line[start : end+2], nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}
