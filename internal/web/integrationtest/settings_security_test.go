package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestBootstrapOmitsStoredAPIKeys_whenSettingsAreLoaded(t *testing.T) {
	// Given stored settings and a preset containing three distinct credentials.
	server, _, repository := newTestServer(t, &stubFetcher{})
	upstream := compatibleLLM(t, "unused")
	configureLLM(t, repository, upstream.URL)
	if _, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKey: "preset-secret",
	}); err != nil {
		t.Fatalf("create preset: %v", err)
	}

	// When the browser loads its bootstrap document.
	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then no credential field or plaintext secret crosses the HTTP boundary.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{`"llm_api_key":`, `"nvd_api_key":`, `"api_key":`, "test-key", "test-nvd-key", "preset-secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("bootstrap response contains %q: %s", forbidden, body)
		}
	}
	var response struct {
		Settings struct {
			LLMConfigured bool `json:"llm_api_key_configured"`
			NVDConfigured bool `json:"nvd_api_key_configured"`
		} `json:"settings"`
		Presets []struct {
			Model      string `json:"model"`
			Configured bool   `json:"api_key_configured"`
		} `json:"llm_presets"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if !response.Settings.LLMConfigured || !response.Settings.NVDConfigured {
		t.Fatalf("configured flags = LLM %t / NVD %t, want true", response.Settings.LLMConfigured, response.Settings.NVDConfigured)
	}
	for _, preset := range response.Presets {
		if preset.Model == "qwen3:8b" && !preset.Configured {
			t.Fatal("saved preset key is not represented by a non-secret configured flag")
		}
	}
}

func TestSaveSettingsPreservesStoredAPIKeys_whenKeyFieldsAreBlank(t *testing.T) {
	// Given settings with persisted LLM and NVD credentials.
	server, _, repository := newTestServer(t, &stubFetcher{})
	upstream := compatibleLLM(t, "unused")
	configureLLM(t, repository, upstream.URL)
	draft, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMAPIKey = ""
	draft.NVDAPIKey = ""
	draft.LLMTimeout = 75
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the user saves other fields while both password inputs stay empty.
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the existing keys remain and the response still reveals no secret field.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	saved, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMAPIKey != "test-key" || saved.NVDAPIKey != "test-nvd-key" {
		t.Fatalf("saved keys = %q / %q, want existing credentials", saved.LLMAPIKey, saved.NVDAPIKey)
	}
	if saved.LLMTimeout != 75 {
		t.Fatalf("timeout = %d, want 75", saved.LLMTimeout)
	}
	for _, forbidden := range []string{`"llm_api_key":`, `"nvd_api_key":`, "test-key", "test-nvd-key"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("settings response contains %q: %s", forbidden, recorder.Body.String())
		}
	}
}

func TestLLMConnectionUsesStoredPresetAPIKey_whenDraftKeyIsBlank(t *testing.T) {
	// Given a preset whose endpoint accepts only its server-specific saved key.
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer preset-key" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"preset-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"ok\":true}"},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(upstream.Close)
	server, _, repository := newTestServer(t, &stubFetcher{})
	if _, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: upstream.URL + "/v1", Model: "preset-model", APIKey: "preset-key",
	}); err != nil {
		t.Fatalf("create preset: %v", err)
	}
	draft, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMBaseURL = upstream.URL + "/v1"
	draft.LLMModel = "preset-model"
	draft.LLMAPIKey = ""
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the connection test is submitted without repopulating the key input.
	request := httptest.NewRequest(http.MethodPost, "/api/llm/test", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the server resolves the matching preset key without returning it to the browser.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestLLMConnectionOmitsStoredAPIKey_whenEndpointIsUnknown(t *testing.T) {
	// Given stored credentials and an endpoint that belongs to neither the settings nor a preset.
	received := make(chan string, 1)
	unknown := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case received <- request.Header.Get("Authorization"):
		default:
		}
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(unknown.Close)
	server, _, repository := newTestServer(t, &stubFetcher{})
	configureLLM(t, repository, "https://api.openai.com")
	draft, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMBaseURL = unknown.URL + "/v1"
	draft.LLMModel = "unknown-model"
	draft.LLMAPIKey = ""
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When a connection test targets that endpoint without supplying a key.
	request := httptest.NewRequest(http.MethodPost, "/api/llm/test", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the stored key is never forwarded to it.
	select {
	case authorization := <-received:
		if strings.Contains(authorization, "test-key") {
			t.Fatalf("unknown endpoint received %q, want no stored credential", authorization)
		}
	default:
		t.Fatalf("unknown endpoint was never contacted")
	}
}

func TestLLMConnectionOmitsActiveAPIKey_whenMatchingPresetKeyIsBlank(t *testing.T) {
	// Given active credentials for one endpoint and a matching keyless preset for another.
	received := make(chan string, 1)
	keyless := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"test","object":"chat.completion","created":1,"model":"keyless-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"ok\":true}"},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(keyless.Close)
	server, _, repository := newTestServer(t, &stubFetcher{})
	configureLLM(t, repository, "https://credentialed.example")
	if _, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: keyless.URL + "/v1", Model: "keyless-model", APIKey: "",
	}); err != nil {
		t.Fatalf("create keyless preset: %v", err)
	}
	draft, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMBaseURL = keyless.URL + "/v1"
	draft.LLMModel = "keyless-model"
	draft.LLMAPIKey = ""
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the keyless preset is tested through the HTTP API.
	request := httptest.NewRequest(http.MethodPost, "/api/llm/test", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the other endpoint's stored credential is not forwarded to it.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	select {
	case authorization := <-received:
		if strings.Contains(authorization, "test-key") {
			t.Fatalf("keyless preset received %q, want no active credential", authorization)
		}
	default:
		t.Fatal("keyless preset endpoint was never contacted")
	}
}

func TestSaveSettingsUsesStoredPresetAPIKey_whenSwitchingWithBlankKey(t *testing.T) {
	// Given a saved preset selected by its endpoint and model.
	server, _, repository := newTestServer(t, &stubFetcher{})
	if _, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKey: "preset-key",
	}); err != nil {
		t.Fatalf("create preset: %v", err)
	}
	draft, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMBaseURL = "http://localhost:11434/v1"
	draft.LLMModel = "qwen3:8b"
	draft.LLMAPIKey = ""
	body, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the preset is saved without copying its key into the browser.
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the active settings use the selected preset's server-side credential.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	saved, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMAPIKey != "preset-key" {
		t.Fatalf("active LLM key = %q, want preset-key", saved.LLMAPIKey)
	}
}
