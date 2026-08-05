package cyberdashboard

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedFrontendIncludesGeneratedLicenseDocuments(t *testing.T) {
	// Given the frontend embedded in the full distribution binary.
	assets, err := EmbeddedFrontend()
	if err != nil {
		t.Fatalf("open embedded frontend: %v", err)
	}

	// When both generated legal documents are read from the embedded filesystem.
	programLicense, err := fs.ReadFile(assets, "legal/LICENSE.txt")
	if err != nil {
		t.Fatalf("read embedded program license: %v", err)
	}
	thirdPartyNotices, err := fs.ReadFile(assets, "legal/THIRD_PARTY_NOTICES.txt")
	if err != nil {
		t.Fatalf("read embedded third-party notices: %v", err)
	}
	rootLicense, err := os.ReadFile("LICENSE")
	if err != nil {
		t.Fatalf("read root license: %v", err)
	}

	// Then generation preserved the project license and discovered runtime and bundled dependencies.
	if string(programLicense) != string(rootLicense) {
		t.Fatal("generated program license differs from root LICENSE")
	}
	for _, expected := range []string{"static/jquery-4.0.0.min.js", "github.com/openai/openai-go/v3 ", "modernc.org/sqlite "} {
		if !strings.Contains(string(thirdPartyNotices), expected) {
			t.Fatalf("third-party notices do not contain %q", expected)
		}
	}
}
