package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLicenseDocumentsRejectsOversizedFile(t *testing.T) {
	// Given a license document larger than the generator's accepted input boundary.
	directory := t.TempDir()
	path := filepath.Join(directory, "LICENSE")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", int(maximumLicenseDocumentBytes)+1)), 0o644); err != nil {
		t.Fatalf("write oversized license: %v", err)
	}

	// When the dependency license directory is read.
	_, err := readLicenseDocuments(directory)

	// Then the document is rejected instead of being read without a memory bound.
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("read oversized license error = %v, want size-limit rejection", err)
	}
}
