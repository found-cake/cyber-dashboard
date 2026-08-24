package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestReadLicenseDocumentsFromFindsPackageLicense_whenModuleRootUnlicensed(t *testing.T) {
	// Given a module whose imported package carries the only license document.
	root := t.TempDir()
	packageDirectory := filepath.Join(root, "pkg", "rssjson")
	if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
		t.Fatalf("create package directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packageDirectory, "LICENSE.code"), []byte("MIT terms\n"), 0o644); err != nil {
		t.Fatalf("write package license: %v", err)
	}

	// When license discovery checks the module root followed by the imported package.
	documents, err := readLicenseDocumentsFrom([]string{root, packageDirectory})

	// Then the package-scoped license is returned.
	if err != nil || len(documents) != 1 || documents[0].Name != "LICENSE.code" || documents[0].Text != "MIT terms\n" {
		t.Fatalf("license documents = %+v, err = %v", documents, err)
	}
}
