//go:build browser

package browsertest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/found-cake/cyber-dashboard/api"
)

func TestSettingsLicenseDialogShowsProgramAndThirdPartyLicenses(t *testing.T) {
	server := newSettingsSecurityBrowserServer(t, make(chan api.Settings, 1))
	viewports := []struct {
		width  int64
		height int64
	}{{375, 812}, {768, 900}, {1280, 900}}

	for _, viewport := range viewports {
		t.Run(fmt.Sprintf("%dpx", viewport.width), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			t.Cleanup(cancel)
			browser, browserCancel := chromedp.NewContext(ctx)
			t.Cleanup(browserCancel)

			if err := chromedp.Run(browser,
				chromedp.EmulateViewport(viewport.width, viewport.height),
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible(`#main-content .empty-state`),
			); err != nil {
				t.Fatalf("initialize dashboard: %v", err)
			}
			openSettingsPage(t, browser, viewport.width)
			if err := chromedp.Run(browser,
				chromedp.ScrollIntoView(`#open-license-modal`),
				chromedp.WaitVisible(`#open-license-modal`),
			); err != nil {
				t.Fatalf("find license action: %v", err)
			}
			captureLicenseScreenshot(t, browser, viewport.width, "settings-license-action")

			if err := chromedp.Run(browser,
				chromedp.Click(`#open-license-modal`),
				chromedp.WaitVisible(`#license-document`),
				chromedp.Poll(`document.querySelector('#license-document').textContent.includes('MIT License')`, nil),
			); err != nil {
				t.Fatalf("open program license: %v", err)
			}
			captureLicenseScreenshot(t, browser, viewport.width, "program-license")

			if err := chromedp.Run(browser,
				chromedp.Click(`[data-license-tab="thirdParty"]`),
				chromedp.Poll(`document.querySelector('#license-document').textContent.includes('openai-go') && document.querySelector('#license-document').textContent.includes('jQuery')`, nil),
			); err != nil {
				t.Fatalf("open third-party licenses: %v", err)
			}
			captureLicenseScreenshot(t, browser, viewport.width, "third-party-licenses")

			var result struct {
				DialogOpen       bool   `json:"dialogOpen"`
				SelectedTab      string `json:"selectedTab"`
				PanelLabelledBy  string `json:"panelLabelledBy"`
				ThirdPartyNotice string `json:"thirdPartyNotice"`
			}
			if err := chromedp.Run(browser, chromedp.Evaluate(`({
				dialogOpen: document.querySelector('#modal-root [role="dialog"]') !== null,
				selectedTab: document.querySelector('[data-license-tab][aria-selected="true"]').dataset.licenseTab,
				panelLabelledBy: document.querySelector('#license-document').getAttribute('aria-labelledby'),
				thirdPartyNotice: document.querySelector('#license-document').textContent
			})`, &result)); err != nil {
				t.Fatalf("inspect license dialog: %v", err)
			}
			if !result.DialogOpen || result.SelectedTab != "thirdParty" || result.PanelLabelledBy != "license-tab-third-party" {
				t.Fatalf("license dialog state = %+v", result)
			}
			if !strings.Contains(result.ThirdPartyNotice, "Apache License") || !strings.Contains(result.ThirdPartyNotice, "Redistribution and use") {
				t.Fatalf("third-party notice is incomplete: %.120s", result.ThirdPartyNotice)
			}

			if err := chromedp.Run(browser,
				chromedp.KeyEvent(kb.Escape),
				chromedp.WaitNotPresent(`#license-document`),
				chromedp.Poll(`document.activeElement && document.activeElement.id === 'open-license-modal'`, nil),
			); err != nil {
				t.Fatalf("close license dialog and restore focus: %v", err)
			}
		})
	}
}

func captureLicenseScreenshot(t *testing.T, browser context.Context, width int64, state string) {
	t.Helper()
	directory := os.Getenv("CYBER_DASHBOARD_VISUAL_QA_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create license visual QA directory: %v", err)
	}
	var screenshot []byte
	if err := chromedp.Run(browser,
		chromedp.Poll(`!document.querySelector('.modal') || document.querySelector('.modal').getAnimations().every(animation => animation.playState === 'finished')`, nil),
		chromedp.CaptureScreenshot(&screenshot),
	); err != nil {
		t.Fatalf("capture %s at %dpx: %v", state, width, err)
	}
	if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%d-%s.png", width, state)), screenshot, 0o644); err != nil {
		t.Fatalf("write %s at %dpx: %v", state, width, err)
	}
}
