//go:build browser

package browsertest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cdpinput "github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

func openSettingsPage(t *testing.T, browser context.Context, width int64) {
	t.Helper()
	if width <= 900 {
		if err := chromedp.Run(browser,
			chromedp.WaitVisible(`#menu-button`),
			chromedp.Click(`#menu-button`),
			chromedp.Poll(`(() => {
				const element = document.querySelector('[data-view="settings"]');
				const rect = element.getBoundingClientRect();
				const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
				return rect.left >= 0 && rect.right <= innerWidth && (hit === element || element.contains(hit));
			})()`, nil),
		); err != nil {
			t.Fatalf("open mobile drawer: %v", err)
		}
	}
	if err := chromedp.Run(browser,
		chromedp.Click(`[data-view="settings"]`),
		chromedp.WaitVisible(`#llm-api-key`),
	); err != nil {
		t.Fatalf("navigate to settings: %v", err)
	}
	if width <= 900 {
		if err := chromedp.Run(browser, chromedp.Poll(`(() => {
			const sidebar = document.querySelector('#sidebar');
			return !document.querySelector('.app-shell').classList.contains('is-drawer-open') &&
				!document.querySelector('.main-column').inert && sidebar.getBoundingClientRect().right <= 0;
		})()`, nil)); err != nil {
			t.Fatalf("wait for mobile drawer to close: %v", err)
		}
	}
}

func captureSettingsEvidence(t *testing.T, browser context.Context, serverURL string, width, height int64) {
	t.Helper()
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create visual QA directory: %v", err)
	}
	warmSettingsCapture(t, browser, width)
	nvdScreenshot := captureSettingsTarget(t, browser, "#nvd-api-key", width, height)

	llmRoot, rootCancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(rootCancel)
	llmBrowser, browserCancel := chromedp.NewContext(llmRoot)
	t.Cleanup(browserCancel)
	if err := chromedp.Run(llmBrowser,
		chromedp.EmulateViewport(width, height),
		chromedp.Navigate(serverURL),
		chromedp.WaitVisible(`#main-content .empty-state`),
		chromedp.Evaluate(`document.querySelector('[data-view="settings"]').click()`, nil),
		chromedp.WaitVisible(`#llm-api-key`),
	); err != nil {
		t.Fatalf("initialize LLM evidence page at %dpx: %v", width, err)
	}
	warmSettingsCapture(t, llmBrowser, width)
	llmScreenshot := captureSettingsTarget(t, llmBrowser, "#llm-api-key", width, height)
	writeSettingsScreenshot(t, directory, "nvd-api-key", width, nvdScreenshot)
	writeSettingsScreenshot(t, directory, "llm-api-key", width, llmScreenshot)
}

func writeSettingsScreenshot(t *testing.T, directory, name string, width int64, screenshot []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%d-%s.png", width, name)), screenshot, 0o644); err != nil {
		t.Fatalf("write visual QA screenshot: %v", err)
	}
}

func warmSettingsCapture(t *testing.T, browser context.Context, width int64) {
	t.Helper()
	var warmup []byte
	if err := chromedp.Run(browser, chromedp.CaptureScreenshot(&warmup)); err != nil {
		t.Fatalf("warm visual QA capture at %dpx: %v", width, err)
	}
}

func captureSettingsTarget(t *testing.T, browser context.Context, target string, width, height int64) []byte {
	t.Helper()
	var screenshot []byte
	delta := float64(1)
	if target == "#llm-api-key" {
		if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
				const rect = document.querySelector("#llm-api-key").getBoundingClientRect();
				return rect.top - (innerHeight + 68) / 2 - 32;
			})()`, &delta)); err != nil {
			t.Fatalf("measure scroll to %s at %dpx: %v", target, width, err)
		}
	}
	if err := chromedp.Run(browser,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return cdpinput.DispatchMouseEvent(cdpinput.MouseWheel, float64(width)/2, float64(height)/2).WithDeltaY(delta).Do(ctx)
		}),
	); err != nil {
		t.Fatalf("scroll to %s at %dpx: %v", target, width, err)
	}
	if target == "#llm-api-key" {
		if err := chromedp.Run(browser, chromedp.Poll(`(() => {
				const rect = document.querySelector("#llm-api-key").getBoundingClientRect();
				return rect.top >= 68 && rect.bottom <= innerHeight;
			})()`, nil)); err != nil {
			t.Fatalf("scroll to %s at %dpx: %v", target, width, err)
		}
	}
	if err := chromedp.Run(browser, chromedp.Poll(`(() => {
		const rect = document.querySelector(".topbar").getBoundingClientRect();
		return rect.top === 0 && rect.bottom === 68 && document.elementFromPoint(rect.left + 8, 8).closest(".topbar");
	})()`, nil)); err != nil {
		t.Fatalf("keep topbar visible while capturing %s at %dpx: %v", target, width, err)
	}
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`window.__settingsCaptureFrames = 0;
			requestAnimationFrame(() => requestAnimationFrame(() => { window.__settingsCaptureFrames = 2; }));`, nil),
		chromedp.Poll(`window.__settingsCaptureFrames === 2`, nil),
		chromedp.Screenshot(`.app-shell`, &screenshot, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("capture %s at %dpx: %v", target, width, err)
	}
	return screenshot
}
