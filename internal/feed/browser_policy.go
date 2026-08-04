package feed

import (
	"context"
	"fmt"
	"net/url"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type chromiumDocumentGuard struct {
	policy      articleURLPolicy
	context     context.Context
	executor    cdp.Executor
	mainFrameID cdp.FrameID
	violations  chan error
	enabled     bool
}

func newChromiumDocumentGuard(sourceHost string) (*chromiumDocumentGuard, error) {
	policy, err := parseArticleURLPolicy(sourceHost)
	if err != nil {
		return nil, fmt.Errorf("invalid Chromium source host: %w", err)
	}
	return &chromiumDocumentGuard{policy: policy, violations: make(chan error, 1)}, nil
}

func (g *chromiumDocumentGuard) enable(ctx context.Context) error {
	frameTree, err := page.GetFrameTree().Do(ctx)
	if err != nil {
		return fmt.Errorf("read Chromium frame tree: %w", err)
	}
	if frameTree == nil || frameTree.Frame == nil {
		return fmt.Errorf("read Chromium frame tree: main frame is missing")
	}
	g.context = ctx
	g.executor = cdp.ExecutorFromContext(ctx)
	g.mainFrameID = frameTree.Frame.ID
	chromedp.ListenTarget(ctx, g.handleEvent)
	patterns := []*fetch.RequestPattern{{
		URLPattern:   "*",
		ResourceType: network.ResourceTypeDocument,
		RequestStage: fetch.RequestStageRequest,
	}}
	if err := fetch.Enable().WithPatterns(patterns).Do(ctx); err != nil {
		return fmt.Errorf("enable Chromium document guard: %w", err)
	}
	g.enabled = true
	return nil
}

func (g *chromiumDocumentGuard) handleEvent(event any) {
	paused, ok := event.(*fetch.EventRequestPaused)
	if !ok {
		return
	}
	go g.resolveRequest(paused)
}

func (g *chromiumDocumentGuard) resolveRequest(paused *fetch.EventRequestPaused) {
	ctx := cdp.WithExecutor(g.context, g.executor)
	if paused.FrameID == g.mainFrameID {
		parsed, err := url.Parse(paused.Request.URL)
		if err != nil {
			g.rejectRequest(ctx, paused.RequestID, fmt.Errorf("parse Chromium document URL: %w", err))
			return
		}
		if err := g.policy.validate(parsed, false); err != nil {
			g.rejectRequest(ctx, paused.RequestID, fmt.Errorf("reject Chromium document navigation: %w", err))
			return
		}
	}
	if err := fetch.ContinueRequest(paused.RequestID).Do(ctx); err != nil {
		g.recordViolation(fmt.Errorf("continue Chromium document navigation: %w", err))
	}
}

func (g *chromiumDocumentGuard) rejectRequest(ctx context.Context, requestID fetch.RequestID, violation error) {
	g.recordViolation(violation)
	if err := fetch.FailRequest(requestID, network.ErrorReasonBlockedByClient).Do(ctx); err != nil {
		g.recordViolation(fmt.Errorf("block Chromium document navigation: %w", err))
	}
}

func (g *chromiumDocumentGuard) recordViolation(err error) {
	select {
	case g.violations <- err:
	default:
	}
}

func (g *chromiumDocumentGuard) takeViolation() error {
	select {
	case err := <-g.violations:
		return err
	default:
		return nil
	}
}

func (g *chromiumDocumentGuard) validateFinalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse final Chromium URL: %w", err)
	}
	if err := g.policy.validate(parsed, false); err != nil {
		return fmt.Errorf("reject final Chromium URL: %w", err)
	}
	return nil
}

func (g *chromiumDocumentGuard) close(ctx context.Context) error {
	if !g.enabled {
		return nil
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return fetch.Disable().Do(ctx)
	}))
}
