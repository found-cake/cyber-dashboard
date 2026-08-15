package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestRunKeepsGeneratedPasswordOutOfStructuredLogs(t *testing.T) {
	// Given a first startup with stderr captured separately from structured logs.
	t.Setenv("CYBER_DASHBOARD_ADDR", "127.0.0.1:0")
	t.Setenv("CYBER_DASHBOARD_DATA_DIR", t.TempDir())
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("capture stderr: %v", err)
	}
	previousStderr := os.Stderr
	os.Stderr = writer
	writerClosed := false
	t.Cleanup(func() {
		os.Stderr = previousStderr
		if !writerClosed {
			if err := writer.Close(); err != nil {
				t.Errorf("close captured stderr writer: %v", err)
			}
		}
		if err := reader.Close(); err != nil {
			t.Errorf("close captured stderr reader: %v", err)
		}
	})
	generatedPasswordAttribute := make(chan bool, 1)
	ready := make(chan struct{})
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(&initialPasswordOutputHandler{
		generatedPasswordAttribute: generatedPasswordAttribute,
		ready:                      ready,
	}))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	// When the application generates its initial administrator password.
	go func() {
		done <- Run(ctx, fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}})
	}()
	select {
	case <-ready:
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("dashboard did not start")
	}
	if err := <-done; err != nil {
		t.Fatalf("run dashboard: %v", err)
	}
	os.Stderr = previousStderr
	if err := writer.Close(); err != nil {
		t.Fatalf("close captured stderr: %v", err)
	}
	writerClosed = true
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stderr: %v", err)
	}

	// Then logs contain no password field and stderr carries the one-time credential.
	if <-generatedPasswordAttribute {
		t.Fatal("generated password was attached to a structured log record")
	}
	const prefix = "Initial dashboard password: "
	password := strings.TrimSpace(strings.TrimPrefix(string(output), prefix))
	if !strings.HasPrefix(string(output), prefix) || len(password) < 20 {
		t.Fatalf("initial password stderr output is missing or malformed")
	}
}

type initialPasswordOutputHandler struct {
	slog.Handler
	generatedPasswordAttribute chan<- bool
	ready                      chan<- struct{}
}

func (handler *initialPasswordOutputHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (handler *initialPasswordOutputHandler) Handle(_ context.Context, record slog.Record) error {
	switch record.Message {
	case "generated initial dashboard password":
		hasPassword := false
		record.Attrs(func(attribute slog.Attr) bool {
			hasPassword = hasPassword || attribute.Key == "password"
			return true
		})
		handler.generatedPasswordAttribute <- hasPassword
	case "cyber dashboard listening":
		close(handler.ready)
	}
	return nil
}

func (handler *initialPasswordOutputHandler) WithAttrs([]slog.Attr) slog.Handler {
	return handler
}

func (handler *initialPasswordOutputHandler) WithGroup(string) slog.Handler {
	return handler
}
