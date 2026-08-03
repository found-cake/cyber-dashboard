package feed

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const chromiumUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

type ChromiumBodyLoader struct {
	context context.Context
	cancel  context.CancelFunc
	mutex   sync.Mutex
}

func NewChromiumBodyLoader(parent context.Context) *ChromiumBodyLoader {
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.UserAgent(chromiumUserAgent),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
	)
	allocatorContext, allocatorCancel := chromedp.NewExecAllocator(parent, options...)
	browserContext, browserCancel := chromedp.NewContext(allocatorContext)
	return &ChromiumBodyLoader{
		context: browserContext,
		cancel: func() {
			browserCancel()
			allocatorCancel()
		},
	}
}

func (l *ChromiumBodyLoader) Close() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.cancel()
}

func (l *ChromiumBodyLoader) Load(ctx context.Context, articleURL string) (string, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if err := chromedp.Run(l.context); err != nil {
		return "", fmt.Errorf("start shared Chromium session: %w", err)
	}
	tabContext, timeoutCancel := context.WithTimeout(l.context, 60*time.Second)
	defer timeoutCancel()
	stopCancellation := context.AfterFunc(ctx, timeoutCancel)
	defer stopCancellation()
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
			if _, err := page.AddScriptToEvaluateOnNewDocument(`Object.defineProperty(navigator, "webdriver", {get: () => undefined});`).Do(ctx); err != nil {
				return err
			}
			_, _, errorText, isDownload, err := page.Navigate(articleURL).Do(ctx)
			if err != nil {
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
		waitContext, waitCancel := context.WithTimeout(tabContext, 50*time.Second)
		defer waitCancel()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
	waitForBody:
		for {
			body = ""
			loadErr = chromedp.Run(waitContext, chromedp.Evaluate(bodyExpression, &body))
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
	if loadErr != nil {
		var pageState string
		_ = chromedp.Run(tabContext, chromedp.Evaluate(`(() => {
  const text = document.body ? document.body.innerText.slice(0, 200) : "";
  return "title=" + document.title + " url=" + location.href + " text=" + text;
})()`, &pageState))
		return "", fmt.Errorf("load article in Chromium: %w (%s)", loadErr, strings.TrimSpace(pageState))
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("Chromium article body is empty")
	}
	return body, nil
}
