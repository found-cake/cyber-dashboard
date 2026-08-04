//go:build browser

package web_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestCVERefreshVisualEvidence(t *testing.T) {
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		t.Skip("CYBER_DASHBOARD_VISUAL_QA_DIR is not configured")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	server, _, release, _ := newCVERefreshServer(t)
	t.Cleanup(release)
	for _, width := range []int64{375, 768, 1280} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		browser, browserCancel := chromedp.NewContext(ctx)
		var screenshot []byte
		err := chromedp.Run(browser,
			chromedp.EmulateViewport(width, 900),
			chromedp.Navigate(server.URL+"/#cves"),
			chromedp.WaitVisible("#refresh-cves"),
			chromedp.CaptureScreenshot(&screenshot),
		)
		browserCancel()
		cancel()
		if err != nil {
			t.Fatalf("capture CVE explorer at %dpx: %v", width, err)
		}
		path := filepath.Join(directory, fmt.Sprintf("cve-refresh-%d.png", width))
		if err := os.WriteFile(path, screenshot, 0o644); err != nil {
			t.Fatalf("write CVE screenshot at %dpx: %v", width, err)
		}
	}
}
