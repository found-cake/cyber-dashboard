package body

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sync/atomic"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// discardedResourceTypes never contribute to the extracted article text, so they are
// failed at the request stage instead of downloaded. Stylesheets and scripts are kept:
// CSS decides what innerText sees, and many articles render their body with JS.
//
// This blocks at the network layer rather than through Blink settings on purpose. Turning
// images off in the renderer is a measurable deviation from a real browser and would work
// against the user-agent and webdriver masking the loader already does; a failed request
// looks like an ad blocker or a flaky network instead.
var discardedResourceTypes = []network.ResourceType{
	network.ResourceTypeImage,
	network.ResourceTypeMedia,
	network.ResourceTypeFont,
}

type chromiumDocumentGuard struct {
	policy      articleURLPolicy
	context     context.Context
	executor    cdp.Executor
	mainFrameID cdp.FrameID
	violations  chan error
	enabled     bool
	// discarded counts requests dropped by discardedResourceTypes, so tests can assert the
	// bytes were never fetched rather than inferring it from timing.
	discarded atomic.Int64
}

func isDiscardedResourceType(resourceType network.ResourceType) bool {
	return slices.Contains(discardedResourceTypes, resourceType)
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
	for _, resourceType := range discardedResourceTypes {
		patterns = append(patterns, &fetch.RequestPattern{
			URLPattern:   "*",
			ResourceType: resourceType,
			RequestStage: fetch.RequestStageRequest,
		})
	}
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
	if isDiscardedResourceType(paused.ResourceType) {
		// Not a policy violation: the article simply does not need these bytes, so the
		// request is dropped without failing the load.
		if err := fetch.FailRequest(paused.RequestID, network.ErrorReasonBlockedByClient).Do(ctx); err == nil {
			g.discarded.Add(1)
		}
		return
	}
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
	if err := fetch.ContinueRequest(paused.RequestID).Do(ctx); err != nil && !isSettledInterception(err) {
		g.recordViolation(fmt.Errorf("continue Chromium document navigation: %w", err))
	}
}

func isSettledInterception(err error) bool {
	var protocolError *cdproto.Error
	return errors.As(err, &protocolError) && protocolError.Code == -32602 && protocolError.Message == "Invalid InterceptionId."
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
