package cmd_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestEmbeddedEntrypointServesFrontend_whenBinaryStarts(t *testing.T) {
	// Given the distribution command with its embedded frontend.
	commandPackage := "./cmd/cyber-dashboard-full"

	// When the compiled command starts.
	baseURL := startEntrypoint(t, commandPackage, nil)

	// Then health, HTML, and JavaScript are served by that binary.
	assertResponseContains(t, baseURL+"/healthz", `"status":"ok"`)
	assertResponseContains(t, baseURL+"/", "Cyber Dashboard")
	assertResponseContains(t, baseURL+"/app.js", "renderDashboard")
	assertResponseContains(t, baseURL+"/legal/THIRD_PARTY_NOTICES.txt", "github.com/openai/openai-go/v3")
}

func TestDiskEntrypointServesFrontend_whenStaticDirectoryIsConfigured(t *testing.T) {
	// Given the development command and an external static directory.
	staticDirectory, err := filepath.Abs(filepath.Join("..", "static"))
	if err != nil {
		t.Fatalf("resolve static directory: %v", err)
	}

	// When the compiled command starts with that directory.
	baseURL := startEntrypoint(t, "./cmd/cyber-dashboard-server-only", []string{
		"CYBER_DASHBOARD_STATIC_DIR=" + staticDirectory,
	})

	// Then it serves the same frontend without relying on embedded files.
	assertResponseContains(t, baseURL+"/healthz", `"status":"ok"`)
	assertResponseContains(t, baseURL+"/", "Cyber Dashboard")
	assertResponseContains(t, baseURL+"/styles.css", "--sidebar-width")
	assertResponseContains(t, baseURL+"/legal/THIRD_PARTY_NOTICES.txt", "github.com/openai/openai-go/v3")
}

func TestEmbeddedEntrypointKeepsSettingsPrivate_whenFreshProfileStarts(t *testing.T) {
	// Given the distribution command with a fresh data directory and no saved browser state.
	baseURL := startEntrypoint(t, "./cmd/cyber-dashboard-full", nil)

	// When the initial document and bootstrap response are loaded.
	assertResponseContains(t, baseURL+"/", `<html lang="en"`)
	response, err := (&http.Client{Timeout: 3 * time.Second}).Get(baseURL + "/api/bootstrap")
	if err != nil {
		t.Fatalf("GET bootstrap: %v", err)
	}
	defer response.Body.Close()
	var bootstrap api.Bootstrap
	if err := json.NewDecoder(response.Body).Decode(&bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}

	// Then the public display language is available without exposing administrator settings.
	if response.StatusCode != http.StatusOK || bootstrap.Settings.Language != "en" {
		t.Fatalf("bootstrap status/language = %d/%q, want 200/en", response.StatusCode, bootstrap.Settings.Language)
	}
	if !bootstrap.Auth.Enabled || bootstrap.Auth.Authenticated || len(bootstrap.Sources) != 0 || len(bootstrap.LLMPresets) != 0 {
		t.Fatalf("public bootstrap exposed administrator settings: %+v", bootstrap)
	}
}

func TestDiskEntrypointFails_whenStaticDirectoryIsMissing(t *testing.T) {
	// Given a compiled development command and a missing static directory.
	binary := buildEntrypoint(t, "./cmd/cyber-dashboard-server-only")
	missing := filepath.Join(t.TempDir(), "missing")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary)
	command.Dir = filepath.Join("..")
	command.Env = append(os.Environ(),
		"CYBER_DASHBOARD_DATA_DIR="+t.TempDir(),
		"CYBER_DASHBOARD_ADDR=127.0.0.1:0",
		"CYBER_DASHBOARD_STATIC_DIR="+missing,
	)

	// When the command is started.
	output, err := command.CombinedOutput()

	// Then startup fails with an actionable static-assets error.
	if err == nil {
		t.Fatalf("command succeeded, output = %s", output)
	}
	if !strings.Contains(string(output), "open static assets") {
		t.Fatalf("output = %s, want static-assets error", output)
	}
}

func startEntrypoint(t *testing.T, commandPackage string, extraEnvironment []string) string {
	t.Helper()
	binary := buildEntrypoint(t, commandPackage)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release address: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	command := exec.CommandContext(ctx, binary)
	command.Dir = filepath.Join("..")
	command.Env = append(os.Environ(),
		"CYBER_DASHBOARD_DATA_DIR="+t.TempDir(),
		"CYBER_DASHBOARD_ADDR="+address,
	)
	command.Env = append(command.Env, extraEnvironment...)
	stderr, err := command.StderrPipe()
	if err != nil {
		cancel()
		t.Fatalf("capture stderr: %v", err)
	}
	if err := command.Start(); err != nil {
		cancel()
		t.Fatalf("start command: %v", err)
	}
	ready := make(chan string, 1)
	go scanUntilReady(stderr, ready)
	select {
	case output := <-ready:
		if !strings.Contains(output, "cyber dashboard listening") {
			cancel()
			_ = command.Wait()
			t.Fatalf("command did not start:\n%s", output)
		}
	case <-ctx.Done():
		cancel()
		_ = command.Wait()
		t.Fatal("command startup timed out")
	}
	t.Cleanup(func() {
		cancel()
		_ = command.Wait()
	})
	return "http://" + address
}

func buildEntrypoint(t *testing.T, commandPackage string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), filepath.Base(commandPackage))
	command := exec.Command("go", "build", "-o", binary, commandPackage)
	command.Dir = filepath.Join("..")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", commandPackage, err, output)
	}
	return binary
}

func scanUntilReady(reader io.Reader, ready chan<- string) {
	scanner := bufio.NewScanner(reader)
	var output strings.Builder
	for scanner.Scan() {
		output.WriteString(scanner.Text())
		output.WriteByte('\n')
		if strings.Contains(scanner.Text(), "cyber dashboard listening") {
			ready <- output.String()
			return
		}
	}
	ready <- output.String()
}

func assertResponseContains(t *testing.T, endpoint, expected string) {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(endpoint)
	if err != nil {
		t.Fatalf("GET %s: %v", endpoint, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", endpoint, err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), expected) {
		t.Fatalf("GET %s = %d %s, want 200 containing %q", endpoint, response.StatusCode, body, expected)
	}
}
