package body

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	chromiumUserAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
	chromiumArticleTimeout  = 90 * time.Second
	chromiumBodyWaitTimeout = 75 * time.Second
)

func (l *ChromiumBodyLoader) Load(ctx context.Context, articleURL, sourceHost string) (string, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	documentGuard, err := newChromiumDocumentGuard(sourceHost)
	if err != nil {
		return "", err
	}
	if err := l.ensureSession(); err != nil {
		return "", err
	}
	if err := l.startSession(ctx); err != nil {
		return "", err
	}
	sessionContext := l.context
	tabContext, timeoutCancel := context.WithTimeout(sessionContext, chromiumArticleTimeout)
	defer timeoutCancel()
	stopCancellation := context.AfterFunc(ctx, timeoutCancel)
	defer stopCancellation()
	defer func() {
		guardContext, guardCancel := context.WithTimeout(sessionContext, 2*time.Second)
		defer guardCancel()
		if err := documentGuard.close(guardContext); err != nil {
			l.discardSession()
		}
	}()
	var body string
	bodyExpression := `(() => {
  const selectors = [".articleBody", ".article-content", ".article__content", ".article-body", "article", "main"];
  const root = selectors.map(selector => document.querySelector(selector)).find(Boolean) || document.body;
  const text = root?.innerText?.trim() || "";
  return text.length > 300 && !text.includes("Just a moment") ? text : "";
})()`
	loadErr := chromedp.Run(tabContext,
		chromedp.ActionFunc(func(ctx context.Context) error {
			architecture := "x86"
			if strings.HasPrefix(runtime.GOARCH, "arm") {
				architecture = "arm"
			}
			metadata := &emulation.UserAgentMetadata{
				Brands: []*emulation.UserAgentBrandVersion{
					{Brand: "Not_A Brand", Version: "99"},
					{Brand: "Chromium", Version: "151"},
					{Brand: "Google Chrome", Version: "151"},
				},
				FullVersionList: []*emulation.UserAgentBrandVersion{
					{Brand: "Not_A Brand", Version: "99.0.0.0"},
					{Brand: "Chromium", Version: "151.0.0.0"},
					{Brand: "Google Chrome", Version: "151.0.0.0"},
				},
				Platform: "macOS", PlatformVersion: "10.15.7", Architecture: architecture,
				Model: "", Mobile: false, Bitness: "64", Wow64: false,
			}
			if err := emulation.SetUserAgentOverride(chromiumUserAgent).
				WithAcceptLanguage("en-US,en;q=0.9").WithPlatform("MacIntel").
				WithUserAgentMetadata(metadata).Do(ctx); err != nil {
				return err
			}
			// Added once per session: the injection persists on the page, so adding it per
			// article would stack copies that all re-run on every later navigation.
			if l.documentScriptInjections == 0 {
				if _, err := page.AddScriptToEvaluateOnNewDocument(`Object.defineProperty(navigator, "webdriver", {get: () => undefined});`).Do(ctx); err != nil {
					return err
				}
				l.documentScriptInjections++
			}
			if err := documentGuard.enable(ctx); err != nil {
				return err
			}
			_, _, errorText, isDownload, err := page.Navigate(articleURL).Do(ctx)
			if err != nil {
				return err
			}
			if err := documentGuard.takeViolation(); err != nil {
				return err
			}
			if errorText != "" {
				return fmt.Errorf("navigate article: %s", errorText)
			}
			if isDownload {
				return fmt.Errorf("article navigation started a download")
			}
			return nil
		}),
	)
	if loadErr == nil {
		waitContext, waitCancel := context.WithTimeout(tabContext, chromiumBodyWaitTimeout)
		defer waitCancel()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
	waitForBody:
		for {
			if err := documentGuard.takeViolation(); err != nil {
				loadErr = err
				break
			}
			body = ""
			loadErr = chromedp.Run(waitContext, chromedp.Evaluate(bodyExpression, &body))
			if err := documentGuard.takeViolation(); err != nil {
				loadErr = err
				break
			}
			if loadErr == nil && body != "" {
				break
			}
			var protocolError *cdproto.Error
			if loadErr != nil && (!errors.As(loadErr, &protocolError) || protocolError.Code != -32000) {
				break
			}
			select {
			case <-waitContext.Done():
				loadErr = waitContext.Err()
				break waitForBody
			case <-ticker.C:
			}
		}
	}
	if loadErr == nil && body != "" {
		var finalURL string
		loadErr = chromedp.Run(tabContext, chromedp.Evaluate(`location.href`, &finalURL))
		if loadErr == nil {
			loadErr = documentGuard.validateFinalURL(finalURL)
		}
	}
	if loadErr == nil && body != "" {
		if err := chromedp.Run(tabContext, chromedp.ActionFunc(func(ctx context.Context) error {
			if err := page.StopLoading().Do(ctx); err != nil {
				return err
			}
			return page.ResetNavigationHistory().Do(ctx)
		})); err != nil {
			loadErr = fmt.Errorf("release article resources: %w", err)
		}
	}
	if loadErr != nil {
		if err := documentGuard.takeViolation(); err != nil {
			loadErr = err
		}
		var pageState string
		diagnosticContext, diagnosticCancel := context.WithTimeout(l.context, 2*time.Second)
		defer diagnosticCancel()
		if err := chromedp.Run(diagnosticContext, chromedp.Evaluate(`(() => {
  const text = document.body ? document.body.innerText.slice(0, 200) : "";
  return "title=" + document.title + " url=" + location.href + " text=" + text;
})()`, &pageState)); err != nil {
			pageState = "page state unavailable: " + err.Error()
		}
		if !l.sessionIsActive() {
			l.discardSession()
		}
		return "", fmt.Errorf("load article in Chromium: %w (%s)", loadErr, strings.TrimSpace(pageState))
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("Chromium article body is empty")
	}
	return body, nil
}
