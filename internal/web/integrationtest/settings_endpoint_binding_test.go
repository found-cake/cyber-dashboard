package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateLanguagePersistsEnglish_whenUserSelectsEnglish(t *testing.T) {
	// Given a new server whose persisted language is Korean.
	server, _, appSettings := newTestServer(t, &stubFetcher{})

	// When the user selects English through the language preference API.
	request := httptest.NewRequest(http.MethodPatch, "/api/settings/language", strings.NewReader(`{"language":"en"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then English is persisted for background summary generation.
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	value, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if value.Language != "en" {
		t.Fatalf("language = %q, want en", value.Language)
	}
}

func TestSaveSettingsClearsActiveAPIKey_whenEndpointChangesWithoutReplacement(t *testing.T) {
	// Given active credentials for one endpoint and a different keyless endpoint.
	received := make(chan string, 1)
	keyless := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"test","object":"chat.completion","created":1,"model":"keyless-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"ok\":true}"},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(keyless.Close)
	server, _, repository := newTestServer(t, &stubFetcher{})
	configureLLM(t, repository, "https://credentialed.example")
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

	// When the endpoint is saved without a replacement key and tested from persisted settings.
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRecorder := httptest.NewRecorder()
	server.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", saveRecorder.Code, saveRecorder.Body.String())
	}
	testRequest := httptest.NewRequest(http.MethodPost, "/api/llm/test", nil)
	testRecorder := httptest.NewRecorder()
	server.ServeHTTP(testRecorder, testRequest)

	// Then the previous endpoint's credential is cleared before the new endpoint is contacted.
	if testRecorder.Code != http.StatusOK {
		t.Fatalf("test status = %d, body = %s", testRecorder.Code, testRecorder.Body.String())
	}
	select {
	case authorization := <-received:
		if strings.Contains(authorization, "test-key") {
			t.Fatalf("keyless endpoint received %q, want no previous credential", authorization)
		}
	default:
		t.Fatal("keyless endpoint was never contacted")
	}
	saved, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMAPIKey != "" {
		t.Fatalf("saved LLM key = %q, want cleared credential", saved.LLMAPIKey)
	}
}
