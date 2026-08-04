package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
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

type readinessBlockingHandler struct {
	slog.Handler
	logEntered chan struct{}
	releaseLog chan struct{}
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
