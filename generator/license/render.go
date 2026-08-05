package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func renderThirdPartyNotices(modules []moduleNotice, assets []assetNotice) string {
	sort.Slice(modules, func(left, right int) bool { return modules[left].Path < modules[right].Path })
	sort.Slice(assets, func(left, right int) bool { return assets[left].Path < assets[right].Path })

	var output strings.Builder
	output.WriteString("Cyber Dashboard Third-Party Notices\n")
	output.WriteString("===================================\n\n")
	output.WriteString("This file is generated from the binaries' Go dependency graph and bundled frontend license headers.\n")
	output.WriteString("Do not edit this document manually.\n")

	if len(assets) > 0 {
		output.WriteString("\nBundled frontend assets\n-----------------------\n")
		for _, asset := range assets {
			fmt.Fprintf(&output, "\n%s\n%s\n", asset.Path, asset.Notice)
		}
	}

	output.WriteString("\nRuntime dependencies\n--------------------\n")
	for _, module := range modules {
		fmt.Fprintf(&output, "\n%s\n", strings.TrimSpace(module.Path+" "+module.Version))
		for _, license := range module.Licenses {
			fmt.Fprintf(&output, "\n[%s]\n%s", license.Name, license.Text)
			if !strings.HasSuffix(license.Text, "\n") {
				output.WriteByte('\n')
			}
		}
	}
	return output.String()
}

func writeDocuments(directory, program, thirdParty string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create generated license directory: %w", err)
	}
	documents := []struct {
		name    string
		content string
	}{
		{name: "LICENSE.txt", content: program},
		{name: "THIRD_PARTY_NOTICES.txt", content: thirdParty},
	}
	for _, document := range documents {
		if err := os.WriteFile(filepath.Join(directory, document.name), []byte(document.content), 0o644); err != nil {
			return fmt.Errorf("write generated %s: %w", document.name, err)
		}
	}
	return nil
}
