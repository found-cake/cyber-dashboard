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

func TestSaveSettingsAppliesSourceStates_whenTheDraftCarriesThem(t *testing.T) {
	// Given a server whose source with ID 2 is enabled.
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	draft, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMTimeout = 90
	body, err := json.Marshal(api.SaveSettingsRequest{Settings: draft, Sources: []api.SourceState{{ID: 2, Enabled: false}}})
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the settings are saved with the source change attached.
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then the one request persists both the settings and the source state.
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	sources, err := feeds.Sources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if sourceEnabled(t, sources, 2) {
		t.Fatalf("sources = %+v, want source ID 2 disabled", sources)
	}
	saved, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMTimeout != 90 {
		t.Fatalf("timeout = %d, want 90", saved.LLMTimeout)
	}
}

func TestSaveSettingsKeepsEverything_whenASourceStateIsUnknown(t *testing.T) {
	// Given a draft that changes a valid setting alongside a source that does not exist.
	server, feeds, appSettings := newTestServer(t, &stubFetcher{})
	draft, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	draft.LLMTimeout = 90
	body, err := json.Marshal(api.SaveSettingsRequest{
		Settings: draft, Sources: []api.SourceState{{ID: 2, Enabled: false}, {ID: 999, Enabled: false}},
	})
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	// When the save is submitted.
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	// Then neither the sources nor the settings are half applied.
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	sources, err := feeds.Sources(context.Background())
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if !sourceEnabled(t, sources, 2) {
		t.Fatalf("sources = %+v, want source ID 2 still enabled", sources)
	}
	saved, err := appSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if saved.LLMTimeout == 90 {
		t.Fatal("settings were saved even though a source state was rejected")
	}
}

func sourceEnabled(t *testing.T, sources []api.Source, id int64) bool {
	t.Helper()
	for _, source := range sources {
		if source.ID == id {
			return source.Enabled
		}
	}
	t.Fatalf("source ID %d not found in %+v", id, sources)
	return false
}
