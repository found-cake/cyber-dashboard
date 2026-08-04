package settings

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestPresetLifecycleProtectsBuiltInPreset(t *testing.T) {
	// Given a new repository containing the seeded OpenAI preset.
	repository := newPresetTestRepository(t)
	presets, err := repository.Presets(context.Background())
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	if len(presets) != 1 || !presets[0].Builtin {
		t.Fatalf("presets = %+v, want one built-in preset", presets)
	}

	// When a user preset is created from a compatible endpoint.
	created, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:11434/v1/", Model: "qwen3:8b", APIKey: "local-secret",
	})
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}

	// Then its label is derived from the host and only it can be deleted.
	if created.Label != "localhost:11434" || created.Builtin || created.APIKey != "local-secret" {
		t.Fatalf("created preset = %+v", created)
	}
	if err := repository.DeletePreset(context.Background(), presets[0].ID); !errors.Is(err, ErrBuiltinPreset) {
		t.Fatalf("delete built-in error = %v, want ErrBuiltinPreset", err)
	}
	if err := repository.DeletePreset(context.Background(), created.ID); err != nil {
		t.Fatalf("delete user preset: %v", err)
	}
	remaining, err := repository.Presets(context.Background())
	if err != nil {
		t.Fatalf("list remaining presets: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != presets[0].ID {
		t.Fatalf("remaining presets = %+v, want built-in only", remaining)
	}
}

func TestPresetsKeepSeparateEncryptedAPIKeys_whenMultipleServersAreSaved(t *testing.T) {
	// Given two OpenAI-compatible servers with different credentials.
	repository := newPresetTestRepository(t)
	first, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKey: "ollama-secret",
	})
	if err != nil {
		t.Fatalf("create first preset: %v", err)
	}
	second, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:8888/v1", Model: "gemma-3", APIKey: "gateway-secret",
	})
	if err != nil {
		t.Fatalf("create second preset: %v", err)
	}

	// When the saved presets are loaded again.
	presets, err := repository.Presets(context.Background())
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}

	// Then each server returns its own key and SQLite contains no plaintext key.
	wantKeys := map[int64]string{first.ID: "ollama-secret", second.ID: "gateway-secret"}
	for _, preset := range presets {
		if want, exists := wantKeys[preset.ID]; exists && preset.APIKey != want {
			t.Fatalf("preset %d key = %q, want %q", preset.ID, preset.APIKey, want)
		}
	}
	var rawFirst, rawSecond string
	if err := repository.db.QueryRow(`SELECT api_key FROM llm_presets WHERE id = ?`, first.ID).Scan(&rawFirst); err != nil {
		t.Fatalf("read first raw key: %v", err)
	}
	if err := repository.db.QueryRow(`SELECT api_key FROM llm_presets WHERE id = ?`, second.ID).Scan(&rawSecond); err != nil {
		t.Fatalf("read second raw key: %v", err)
	}
	if strings.Contains(rawFirst, "ollama-secret") || strings.Contains(rawSecond, "gateway-secret") {
		t.Fatal("plaintext preset credential found in database")
	}
}

func TestUpdatePresetAPIKeyReplacesOnlySelectedServerCredential(t *testing.T) {
	// Given two saved server presets.
	repository := newPresetTestRepository(t)
	first, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKey: "first-key",
	})
	if err != nil {
		t.Fatalf("create first preset: %v", err)
	}
	second, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:8888/v1", Model: "gemma-3", APIKey: "second-key",
	})
	if err != nil {
		t.Fatalf("create second preset: %v", err)
	}

	// When only the first preset key is updated.
	if err := repository.UpdatePresetAPIKey(context.Background(), first.ID, "updated-key"); err != nil {
		t.Fatalf("update preset key: %v", err)
	}
	presets, err := repository.Presets(context.Background())
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}

	// Then the selected credential changes without altering the other server.
	keys := map[int64]string{}
	for _, preset := range presets {
		keys[preset.ID] = preset.APIKey
	}
	if keys[first.ID] != "updated-key" || keys[second.ID] != "second-key" {
		t.Fatalf("preset keys = %#v", keys)
	}
}

func TestUpdatePresetAPIKeyPreservesCredential_whenValueIsBlank(t *testing.T) {
	// Given a preset with a saved server credential.
	repository := newPresetTestRepository(t)
	preset, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKey: "saved-key",
	})
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}

	// When the update request contains only whitespace.
	if err := repository.UpdatePresetAPIKey(context.Background(), preset.ID, "  "); err != nil {
		t.Fatalf("update preset key: %v", err)
	}
	presets, err := repository.Presets(context.Background())
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}

	// Then the stored key is unchanged.
	for _, candidate := range presets {
		if candidate.ID == preset.ID && candidate.APIKey != "saved-key" {
			t.Fatalf("preset key = %q, want saved-key", candidate.APIKey)
		}
	}
}

func TestCreatePresetRejectsDuplicateEndpointAndModel(t *testing.T) {
	// Given the seeded OpenAI endpoint and model.
	repository := newPresetTestRepository(t)

	// When the same normalized endpoint and model are added again.
	_, err := repository.CreatePreset(context.Background(), api.CreateLLMPresetRequest{
		BaseURL: "https://api.openai.com/v1/", Model: "gpt-4o-mini",
	})

	// Then the duplicate has a stable typed error.
	if !errors.Is(err, ErrDuplicatePreset) {
		t.Fatalf("create duplicate error = %v, want ErrDuplicatePreset", err)
	}
}

func newPresetTestRepository(t *testing.T) *Repository {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "dashboard.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	return repository
}
