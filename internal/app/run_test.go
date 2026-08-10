package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"testing/fstest"
	"time"
)

func TestServeBindsBeforeLoggingReadiness(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release address: %v", err)
	}
	t.Setenv("CYBER_DASHBOARD_ADDR", address)

	logEntered := make(chan struct{})
	releaseLog := make(chan struct{})
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(&readinessBlockingHandler{
		logEntered: logEntered,
		releaseLog: releaseLog,
	}))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	}()

	select {
	case <-logEntered:
	case <-time.After(time.Second):
		close(releaseLog)
		cancel()
		<-done
		t.Fatal("server did not log readiness")
	}

	connection, err := net.DialTimeout("tcp", address, time.Second)
	close(releaseLog)
	if err != nil {
		cancel()
		<-done
		t.Fatalf("connect after readiness log: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Errorf("close readiness connection: %v", err)
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + address + "/healthz")
	if err != nil {
		cancel()
		<-done
		t.Fatalf("GET after releasing readiness log: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Errorf("close readiness response: %v", err)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("stop server: %v", err)
	}
}

func TestTrustedHostsIncludeAddressAndExplicitEnvironmentHost(t *testing.T) {
	// Given a concrete bind address and a separate reverse-proxy hostname.
	t.Setenv("CYBER_DASHBOARD_ADDR", "192.0.2.10:8080")
	t.Setenv("CYBER_DASHBOARD_TRUSTED_HOST", "Dashboard.Example.com:8443")

	// When startup resolves the fixed trusted hosts.
	hosts, err := trustedHostsFromEnvironment()
	if err != nil {
		t.Fatalf("resolve trusted hosts: %v", err)
	}

	// Then both normalized hostnames are retained without their ports.
	if len(hosts) != 2 || hosts[0] != "192.0.2.10" || hosts[1] != "dashboard.example.com" {
		t.Fatalf("trusted hosts = %v, want [192.0.2.10 dashboard.example.com]", hosts)
	}
}

func TestTrustedHostsAcceptIPv6LoopbackAddress(t *testing.T) {
	// Given the IPv6 form of the default loopback binding.
	t.Setenv("CYBER_DASHBOARD_ADDR", "[::1]:8080")
	t.Setenv("CYBER_DASHBOARD_TRUSTED_HOST", "")

	// When startup resolves the fixed trusted hosts.
	hosts, err := trustedHostsFromEnvironment()
	if err != nil {
		t.Fatalf("resolve trusted hosts: %v", err)
	}

	// Then the bracket-free address the listener reports is still a usable host.
	if len(hosts) != 1 || hosts[0] != "::1" {
		t.Fatalf("trusted hosts = %v, want [::1]", hosts)
	}
}

func TestTrustedHostsSkipUnspecifiedAddress_whenBindingAllInterfaces(t *testing.T) {
	// Given the conventional hostless address for binding every interface.
	t.Setenv("CYBER_DASHBOARD_ADDR", ":8080")
	t.Setenv("CYBER_DASHBOARD_TRUSTED_HOST", "")

	// When startup resolves fixed trusted hosts.
	hosts, err := trustedHostsFromEnvironment()
	if err != nil {
		t.Fatalf("resolve trusted hosts: %v", err)
	}

	// Then the wildcard bind does not become an allowed hostname.
	if len(hosts) != 0 {
		t.Fatalf("trusted hosts = %v, want none", hosts)
	}
}

func TestTrustedHostsRejectMultipleEnvironmentHosts(t *testing.T) {
	// Given one environment variable containing a comma-separated host list.
	t.Setenv("CYBER_DASHBOARD_ADDR", "127.0.0.1:8080")
	t.Setenv("CYBER_DASHBOARD_TRUSTED_HOST", "first.example,second.example")

	// When startup parses the trusted host policy.
	_, err := trustedHostsFromEnvironment()

	// Then startup rejects the list because only one additional host is supported.
	if err == nil {
		t.Fatal("trustedHostsFromEnvironment returned nil error")
	}
}

func TestRunWarns_whenWildcardBindingHasNoTrustedHost(t *testing.T) {
	// Given a wildcard listener without the exceptional trusted-host setting.
	t.Setenv("CYBER_DASHBOARD_ADDR", "0.0.0.0:0")
	t.Setenv("CYBER_DASHBOARD_TRUSTED_HOST", "")
	t.Setenv("CYBER_DASHBOARD_DATA_DIR", t.TempDir())
	warningObserved := make(chan struct{})
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(&warningSignalHandler{observed: warningObserved}))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// When the application starts.
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}})
	}()

	// Then the operator receives one actionable warning before using a browser.
	select {
	case <-warningObserved:
		cancel()
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("wildcard trusted-host warning was not logged")
	}
	if err := <-done; err != nil {
		t.Fatalf("run dashboard: %v", err)
	}
}

type readinessBlockingHandler struct {
	slog.Handler
	logEntered chan struct{}
	releaseLog chan struct{}
}

type warningSignalHandler struct {
	slog.Handler
	observed chan struct{}
}

func (handler *warningSignalHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (handler *warningSignalHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Message == "dashboard is bound to all interfaces without a trusted host" {
		close(handler.observed)
	}
	return nil
}

func (handler *warningSignalHandler) WithAttrs([]slog.Attr) slog.Handler {
	return handler
}

func (handler *warningSignalHandler) WithGroup(string) slog.Handler {
	return handler
}

func (handler *readinessBlockingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (handler *readinessBlockingHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Message == "cyber dashboard listening" {
		close(handler.logEntered)
		<-handler.releaseLog
	}
	return nil
}

func (handler *readinessBlockingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return handler
}

func (handler *readinessBlockingHandler) WithGroup(string) slog.Handler {
	return handler
}
