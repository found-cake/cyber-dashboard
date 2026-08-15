//go:build browser

package browsertest

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

const browserPathEnvironment = "CYBER_DASHBOARD_BROWSER_PATH"

type browserViewport struct {
	width  int64
	height int64
}

var responsiveViewports = []browserViewport{{375, 812}, {768, 900}, {1280, 900}}

func newBrowserContext(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()
	root, rootCancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(rootCancel)
	if configuredPath := os.Getenv(browserPathEnvironment); configuredPath != "" {
		browserPath, err := exec.LookPath(configuredPath)
		if err != nil {
			t.Fatalf("resolve %s: %v", browserPathEnvironment, err)
		}
		options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
		options = append(options, chromedp.ExecPath(browserPath))
		allocator, allocatorCancel := chromedp.NewExecAllocator(root, options...)
		t.Cleanup(allocatorCancel)
		root = allocator
	}
	browser, browserCancel := chromedp.NewContext(root)
	t.Cleanup(browserCancel)
	return browser
}
