package feed

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestChromiumBodyLoaderUsesUnifiedExtendedTimeouts(t *testing.T) {
	// Given every Chromium-backed source shares one loader policy.
	wantArticleTimeout := 90 * time.Second
	wantBodyWaitTimeout := 75 * time.Second

	// When the shared timeout policy is inspected.
	articleTimeout := chromiumArticleTimeout
	bodyWaitTimeout := chromiumBodyWaitTimeout

	// Then both Dark Reading and BleepingComputer receive the extended limits.
	if articleTimeout != wantArticleTimeout {
		t.Fatalf("article timeout = %s, want %s", articleTimeout, wantArticleTimeout)
	}
	if bodyWaitTimeout != wantBodyWaitTimeout {
		t.Fatalf("body wait timeout = %s, want %s", bodyWaitTimeout, wantBodyWaitTimeout)
	}
}

func TestChromiumBodyLoaderHonorsCanceledCaller_beforeStartingChromium(t *testing.T) {
	// Given a loader with no valid Chromium executable and an already-canceled call.
	loader := newChromiumBodyLoader(context.Background(),
		chromedp.ExecPath(filepath.Join(t.TempDir(), "missing-chrome")))
	t.Cleanup(loader.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When article loading is requested.
	_, err := loader.Load(ctx, "https://example.com/article", "example.com")

	// Then caller cancellation wins before Chromium startup is attempted.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("load error = %v, want context.Canceled", err)
	}
}

func TestChromiumBodyLoaderHonorsCallerDeadline_duringFirstStartup(t *testing.T) {
	// Given Chromium startup remains pending longer than the caller deadline.
	loader := newChromiumBodyLoader(context.Background(),
		chromedp.ExecPath(os.Args[0]),
		chromedp.ModifyCmdFunc(func(command *exec.Cmd) {
			command.Args = []string{os.Args[0], "-test.run=TestChromiumStartupHelperProcess"}
			command.Env = append(os.Environ(), "CYBER_DASHBOARD_CHROMIUM_STARTUP_HELPER=1")
		}),
		chromedp.WSURLReadTimeout(time.Second),
	)
	t.Cleanup(loader.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// When loading triggers the first Chromium startup.
	startedAt := time.Now()
	_, err := loader.Load(ctx, "https://example.com/article", "example.com")
	elapsed := time.Since(startedAt)

	// Then startup stops at the caller deadline instead of the allocator timeout.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("load error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed >= 750*time.Millisecond {
		t.Fatalf("startup cancellation took %s, want less than 750ms", elapsed)
	}
}

func TestChromiumStartupHelperProcess(t *testing.T) {
	if os.Getenv("CYBER_DASHBOARD_CHROMIUM_STARTUP_HELPER") != "1" {
		return
	}
	time.Sleep(5 * time.Second)
	os.Exit(0)
}

func TestChromiumBodyLoaderDoesNotRecreateSession_afterClose(t *testing.T) {
	// Given a Chromium loader that has already been closed.
	loader := newChromiumBodyLoader(context.Background(),
		chromedp.ExecPath(filepath.Join(t.TempDir(), "missing-chrome")))
	loader.Close()

	// When another article load is attempted.
	_, err := loader.Load(context.Background(), "https://example.com/article", "example.com")

	// Then the loader reports its closed state without starting a new session.
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("load error = %v, want closed-loader error", err)
	}
}
