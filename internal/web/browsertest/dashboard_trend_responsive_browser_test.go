//go:build browser

package browsertest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

func TestTrendChartsAndRangeFitResponsiveViewports(t *testing.T) {
	server, _ := newDashboardPartialRenderServer(t)
	for _, viewport := range responsiveViewports {
		t.Run(fmt.Sprintf("%dx%d", viewport.width, viewport.height), func(t *testing.T) {
			browser := newBrowserContext(t, 20*time.Second)
			var layout struct {
				PageOverflow bool `json:"pageOverflow"`
				RangeVisible bool `json:"rangeVisible"`
				ChartFits    bool `json:"chartFits"`
			}
			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`#collection-trend svg`),
				chromedp.Evaluate(`(() => {
					const range = document.querySelector("#dashboard-range").getBoundingClientRect();
					const chart = document.querySelector("#collection-trend svg").getBoundingClientRect();
					const card = document.querySelector("#collection-trend").closest(".card").getBoundingClientRect();
					return {
						pageOverflow: document.documentElement.scrollWidth > innerWidth,
						rangeVisible: range.left >= 0 && range.right <= innerWidth && range.width > 0,
						chartFits: chart.left >= card.left && chart.right <= card.right + 0.5
					};
				})()`, &layout),
			); err != nil {
				t.Fatalf("load dashboard at %dx%d: %v", viewport.width, viewport.height, err)
			}
			if layout.PageOverflow || !layout.RangeVisible || !layout.ChartFits {
				t.Fatalf("responsive dashboard at %dx%d = %+v", viewport.width, viewport.height, layout)
			}
			captureTrendScreenshot(t, browser, "dashboard-trends", viewport.width)
			if err := chromedp.Run(browser,
				chromedp.Evaluate(`document.querySelector("#collection-trend [data-bucket]").focus()`, nil),
				chromedp.KeyEvent(kb.Tab),
			); err != nil {
				t.Fatalf("focus a trend bucket at %dx%d: %v", viewport.width, viewport.height, err)
			}
			captureTrendScreenshot(t, browser, "dashboard-trends-focus", viewport.width)
			if viewport.width == 375 {
				if err := chromedp.Run(browser,
					chromedp.KeyEvent(kb.Escape),
					chromedp.Evaluate(`document.querySelector(".main-scroll").scrollTop = document.querySelector("#attribution-trend").closest(".card").offsetTop - 16`, nil),
				); err != nil {
					t.Fatalf("scroll to the mobile attribution chart: %v", err)
				}
				captureTrendScreenshot(t, browser, "dashboard-trends-lower", viewport.width)
			}
		})
	}
}

func TestTrendDatesFollowDashboardLanguage(t *testing.T) {
	server := newDashboardBrowserServerForLanguage(t, func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(t, writer, dashboardBrowserFixture(request, 13))
	}, "ko")
	browser := newBrowserContext(t, 20*time.Second)
	var label string
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#collection-trend [data-bucket]`),
		chromedp.Evaluate(`document.querySelector("#collection-trend [data-bucket]").getAttribute("aria-label")`, &label),
	); err != nil {
		t.Fatalf("load the Korean dashboard: %v", err)
	}
	if !strings.Contains(label, "2026년 6월 1일") || strings.Contains(label, "2026-06-01") {
		t.Fatalf("localized trend label = %q", label)
	}
	captureTrendScreenshot(t, browser, "dashboard-trends-ko", 375)
}

func captureTrendScreenshot(t *testing.T, browser context.Context, name string, width int64) {
	t.Helper()
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		return
	}
	var screenshot []byte
	if err := chromedp.Run(browser, chromedp.CaptureScreenshot(&screenshot)); err != nil {
		t.Fatalf("capture %dpx dashboard: %v", width, err)
	}
	if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%s-%d.png", name, width)), screenshot, 0o644); err != nil {
		t.Fatalf("write %dpx dashboard screenshot: %v", width, err)
	}
}
