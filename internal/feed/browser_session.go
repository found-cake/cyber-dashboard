package feed

import (
	"context"
	"fmt"
	"sync"

	"github.com/chromedp/chromedp"
)

type ChromiumBodyLoader struct {
	parent  context.Context
	options []chromedp.ExecAllocatorOption
	context context.Context
	cancel  context.CancelFunc
	mutex   sync.Mutex
	closed  bool
}

func NewChromiumBodyLoader(parent context.Context) *ChromiumBodyLoader {
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	options = append(options,
		chromedp.Flag("headless", "new"),
		chromedp.UserAgent(chromiumUserAgent),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
	)
	return newChromiumBodyLoader(parent, options...)
}

func newChromiumBodyLoader(parent context.Context, options ...chromedp.ExecAllocatorOption) *ChromiumBodyLoader {
	loader := &ChromiumBodyLoader{
		parent:  parent,
		options: append([]chromedp.ExecAllocatorOption{}, options...),
	}
	loader.createSession()
	return loader
}

func (l *ChromiumBodyLoader) Close() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.closed = true
	l.discardSession()
}

func (l *ChromiumBodyLoader) ensureSession() error {
	if l.closed {
		return fmt.Errorf("Chromium loader is closed")
	}
	if l.sessionIsActive() {
		return nil
	}
	l.discardSession()
	if l.parent == nil {
		return fmt.Errorf("Chromium session cannot be recreated")
	}
	if err := l.parent.Err(); err != nil {
		return err
	}
	l.createSession()
	return nil
}

func (l *ChromiumBodyLoader) createSession() {
	allocatorContext, allocatorCancel := chromedp.NewExecAllocator(l.parent, l.options...)
	browserContext, browserCancel := chromedp.NewContext(allocatorContext)
	l.context = browserContext
	l.cancel = func() {
		browserCancel()
		allocatorCancel()
	}
}

func (l *ChromiumBodyLoader) startSession(ctx context.Context) error {
	sessionContext := l.context
	sessionCancel := l.cancel
	cancelFinished := make(chan struct{})
	stopCancellation := context.AfterFunc(ctx, func() {
		sessionCancel()
		close(cancelFinished)
	})
	err := chromedp.Run(sessionContext)
	if !stopCancellation() {
		<-cancelFinished
	}
	if callerErr := ctx.Err(); callerErr != nil {
		l.context = nil
		l.cancel = nil
		return callerErr
	}
	if err != nil {
		l.discardSession()
		return fmt.Errorf("start shared Chromium session: %w", err)
	}
	return nil
}

func (l *ChromiumBodyLoader) sessionIsActive() bool {
	if l.context == nil || l.context.Err() != nil {
		return false
	}
	chromiumContext := chromedp.FromContext(l.context)
	if chromiumContext == nil || chromiumContext.Browser == nil {
		return chromiumContext != nil
	}
	select {
	case <-chromiumContext.Browser.LostConnection:
		return false
	default:
		return true
	}
}

func (l *ChromiumBodyLoader) discardSession() {
	cancel := l.cancel
	l.context = nil
	l.cancel = nil
	if cancel != nil {
		cancel()
	}
}
