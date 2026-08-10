//go:build browser

package browsertest

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
	for _, width := range []int64{375, 768, 1280} {
		server, started, release, _ := newCVERefreshServer(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		browser, browserCancel := chromedp.NewContext(ctx)
		err := chromedp.Run(browser,
			chromedp.EmulateViewport(width, 900),
			chromedp.Navigate(server.URL+"/#cves"),
			chromedp.WaitVisible("#refresh-cves"),
			chromedp.Click("#refresh-cves"),
			chromedp.WaitVisible("#refresh-cves[aria-busy='true']"),
		)
		if err == nil {
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatalf("wait for CVE refresh at %dpx: %v", width, ctx.Err())
			}
		}
		var screenshot []byte
		if err == nil {
			err = chromedp.Run(browser, chromedp.CaptureScreenshot(&screenshot))
		}
		release()
		browserCancel()
		cancel()
		if err != nil {
			t.Fatalf("capture CVE explorer at %dpx: %v", width, err)
		}
		path := filepath.Join(directory, fmt.Sprintf("cve-refresh-running-%d.png", width))
		if err := os.WriteFile(path, screenshot, 0o644); err != nil {
			t.Fatalf("write CVE screenshot at %dpx: %v", width, err)
		}
	}
}
