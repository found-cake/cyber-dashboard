package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderThirdPartyNoticesSortsModulesAndBundledAssets(t *testing.T) {
	// Given dependencies and frontend assets in discovery order rather than display order.
	modules := []moduleNotice{
		{Path: "example.com/zeta", Version: "v2.0.0", Licenses: []licenseDocument{{Name: "LICENSE", Text: "zeta terms"}}},
		{Path: "example.com/alpha", Version: "v1.0.0", Licenses: []licenseDocument{{Name: "COPYING", Text: "alpha terms"}}},
	}
	assets := []assetNotice{
		{Path: "static/zeta.min.js", Notice: "zeta license"},
		{Path: "static/alpha.min.js", Notice: "alpha license"},
	}

	// When a third-party notice is rendered.
	got := renderThirdPartyNotices(modules, assets)

	// Then every discovered item is included in a deterministic order with its license text.
	ordered := []string{
		"static/alpha.min.js", "static/zeta.min.js",
		"example.com/alpha v1.0.0", "alpha terms",
		"example.com/zeta v2.0.0", "zeta terms",
	}
	position := -1
	for _, expected := range ordered {
		next := strings.Index(got[position+1:], expected)
		if next < 0 {
			t.Fatalf("notice does not contain %q", expected)
		}
		position += next + 1
	}
}

func TestReadStandardLibraryLicenseUsesBundledTermsWhenDistributionOmitsLicense(t *testing.T) {
	// Given a Go distribution directory without a LICENSE file.
	goRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(goRoot, "VERSION"), []byte("go1.26.5\n"), 0o644); err != nil {
		t.Fatalf("write Go version fixture: %v", err)
	}

	// When the standard-library license is collected.
	got, err := readStandardLibraryLicense(goRoot)
	if err != nil {
		t.Fatalf("read standard-library license: %v", err)
	}

	// Then the generator falls back to the bundled canonical BSD terms.
	if !strings.Contains(got, "Copyright 2009 The Go Authors") || !strings.Contains(got, "Redistribution and use") {
		t.Fatalf("fallback license is incomplete: %q", got)
	}
}

func TestWriteDocumentsCreatesStaticLicenseFiles(t *testing.T) {
	// Given an empty output directory.
	output := filepath.Join(t.TempDir(), "legal")

	// When generated license documents are written.
	if err := writeDocuments(output, "program terms\n", "third-party terms\n"); err != nil {
		t.Fatalf("write generated documents: %v", err)
	}

	// Then both stable static filenames contain their generated content.
	tests := []struct {
		name string
		want string
	}{
		{name: "LICENSE.txt", want: "program terms\n"},
		{name: "THIRD_PARTY_NOTICES.txt", want: "third-party terms\n"},
	}
	for _, test := range tests {
		got, err := os.ReadFile(filepath.Join(output, test.name))
		if err != nil {
			t.Fatalf("read %s: %v", test.name, err)
		}
		if string(got) != test.want {
			t.Fatalf("%s = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestDiscoverRuntimeModulesFindsPackageLicense_whenModuleRootUnlicensed(t *testing.T) {
	// Given the dashboard module and its package-licensed cyber-news-feed dependency.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root, err := findModuleRoot(ctx)
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	// When runtime module notices are discovered through the real Go package graph.
	modules, err := discoverRuntimeModules(ctx, root)
	if err != nil {
		t.Fatalf("discover runtime modules: %v", err)
	}

	// Then the dependency's package-scoped license is included.
	for _, module := range modules {
		if module.Path != "github.com/found-cake/cyber-news-feed" {
			continue
		}
		for _, license := range module.Licenses {
			if license.Name == "pkg/rssjson/LICENSE.code" {
				return
			}
		}
		t.Fatalf("cyber-news-feed licenses = %+v, want pkg/rssjson/LICENSE.code", module.Licenses)
	}
	t.Fatal("cyber-news-feed module notice not found")
}

func TestReadLicenseDocumentsFromAggregatesUniqueModuleAndPackageDocuments(t *testing.T) {
	// Given a module license plus package-specific documents, including a repeated package directory.
	root := t.TempDir()
	packageA := filepath.Join(root, "pkg", "a")
	packageB := filepath.Join(root, "pkg", "b")
	for path, contents := range map[string]string{
		filepath.Join(root, "LICENSE"):        "module terms",
		filepath.Join(packageA, "NOTICE"):     "package A notice",
		filepath.Join(packageB, "LICENSE"):    "package B terms",
		filepath.Join(packageB, "NOTICE.txt"): "package B notice",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("write license fixture: %v", err)
		}
	}

	// When all directories used by the runtime graph are inspected.
	documents, err := readLicenseDocumentsFrom([]string{root, packageA, packageB, packageB})

	// Then every distinct document is retained once and package paths disambiguate duplicate filenames.
	if err != nil {
		t.Fatalf("read license documents: %v", err)
	}
	want := []licenseDocument{
		{Name: "LICENSE", Text: "module terms"},
		{Name: "pkg/a/NOTICE", Text: "package A notice"},
		{Name: "pkg/b/LICENSE", Text: "package B terms"},
		{Name: "pkg/b/NOTICE.txt", Text: "package B notice"},
	}
	if len(documents) != len(want) {
		t.Fatalf("documents = %+v, want %+v", documents, want)
	}
	for index := range want {
		if documents[index] != want[index] {
			t.Fatalf("documents[%d] = %+v, want %+v", index, documents[index], want[index])
		}
	}
}

func TestReadLicenseDocumentsRejectsSymlink(t *testing.T) {
	// Given a license-named symlink to a file outside the dependency directory.
	directory := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("private contents"), 0o600); err != nil {
		t.Fatalf("write external fixture: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(directory, "NOTICE")); err != nil {
		t.Skipf("create symlink fixture: %v", err)
	}

	// When license documents are read.
	_, err := readLicenseDocuments(directory)

	// Then the linked file is rejected instead of copied into generated notices.
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("read license documents error = %v, want regular-file rejection", err)
	}
}

func TestReadLicenseDocumentRejectsFileReplacedAfterInspection(t *testing.T) {
	// Given a regular license file that is replaced with a different file after inspection.
	directory := t.TempDir()
	path := filepath.Join(directory, "LICENSE")
	if err := os.WriteFile(path, []byte("expected terms"), 0o644); err != nil {
		t.Fatalf("write license fixture: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect license fixture: %v", err)
	}
	replacement := filepath.Join(directory, "replacement")
	if err := os.WriteFile(replacement, []byte("different contents"), 0o600); err != nil {
		t.Fatalf("write replacement fixture: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove inspected license fixture: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("replace inspected license fixture: %v", err)
	}

	// When the inspected path is opened for collection.
	_, err = readLicenseDocument(path, info)

	// Then the changed file is rejected without reading the link target.
	if err == nil || !strings.Contains(err.Error(), "changed after inspection") {
		t.Fatalf("read license document error = %v, want replacement rejection", err)
	}
}

func TestReadLicenseDocumentsFromRejectsPackageOutsideModuleRoot(t *testing.T) {
	// Given a module root and a reported package directory outside that root.
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "LICENSE"), []byte("module terms"), 0o644); err != nil {
		t.Fatalf("write module license: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "NOTICE"), []byte("outside contents"), 0o644); err != nil {
		t.Fatalf("write outside notice: %v", err)
	}

	// When both directories are presented as belonging to one module.
	_, err := readLicenseDocumentsFrom([]string{root, outside})

	// Then the out-of-root directory is rejected before any document is read from it.
	if err == nil || !strings.Contains(err.Error(), "outside module root") {
		t.Fatalf("read license documents error = %v, want outside-root rejection", err)
	}
}
