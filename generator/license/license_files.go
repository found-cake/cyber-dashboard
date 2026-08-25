package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type licenseDocument struct {
	Name string
	Text string
}

var errNoLicenseDocuments = errors.New("no top-level license, copying, or notice file")

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
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		contents, err := readLicenseDocument(filepath.Join(directory, entry.Name()), info)
		if err != nil {
			return nil, err
		}
		result = append(result, licenseDocument{Name: entry.Name(), Text: string(contents)})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w in %s", errNoLicenseDocuments, directory)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result, nil
}

func readLicenseDocument(path string, expected os.FileInfo) ([]byte, error) {
	if !expected.Mode().IsRegular() {
		return nil, fmt.Errorf("license document %s is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		return nil, fmt.Errorf("license document %s changed after inspection", path)
	}
	return io.ReadAll(file)
}

func readLicenseDocumentsFrom(directories []string) ([]licenseDocument, error) {
	if len(directories) == 0 {
		return nil, errNoLicenseDocuments
	}
	root, err := filepath.EvalSymlinks(filepath.Clean(directories[0]))
	if err != nil {
		return nil, err
	}
	documents := make([]licenseDocument, 0, 2)
	seenDirectories := make(map[string]struct{}, len(directories))
	for _, directory := range directories {
		directory, err = filepath.EvalSymlinks(filepath.Clean(directory))
		if err != nil {
			return nil, err
		}
		relativeDirectory, err := filepath.Rel(root, directory)
		if err != nil {
			return nil, err
		}
		if relativeDirectory == ".." || strings.HasPrefix(relativeDirectory, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeDirectory) {
			return nil, fmt.Errorf("license directory %s is outside module root %s", directory, root)
		}
		if _, exists := seenDirectories[directory]; exists {
			continue
		}
		seenDirectories[directory] = struct{}{}
		found, err := readLicenseDocuments(directory)
		if errors.Is(err, errNoLicenseDocuments) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, document := range found {
			if relativeDirectory != "." {
				document.Name = filepath.ToSlash(filepath.Join(relativeDirectory, document.Name))
			}
			documents = append(documents, document)
		}
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("%w in %s or its runtime packages", errNoLicenseDocuments, root)
	}
	sort.Slice(documents, func(left, right int) bool { return documents[left].Name < documents[right].Name })
	return documents, nil
}

func isLicenseFilename(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "COPYING") || strings.HasPrefix(upper, "NOTICE")
}
