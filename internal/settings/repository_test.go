package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
)

func TestRepositoryEncryptsSecrets_whenSettingsAreSaved(t *testing.T) {
	// Given a new settings repository and external API credentials.
	databasePath := filepath.Join(t.TempDir(), "dashboard.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository, err := NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	value := api.Settings{Language: "ko", Accent: "#4f6ef7", LLMBaseURL: "https://api.openai.com/v1", LLMModel: "gpt-4o-mini", LLMAPIKey: "llm-secret", LLMTimeout: 60, NVDAPIKey: "nvd-secret", TimezoneOffsetMinutes: 540}

	// When the settings are saved and read through the feature boundary.
	if err := repository.Save(context.Background(), value); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	loaded, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	// Then callers receive originals while SQLite contains no plaintext secrets.
	if loaded.LLMAPIKey != value.LLMAPIKey || loaded.NVDAPIKey != value.NVDAPIKey {
		t.Fatalf("loaded secrets = %q / %q", loaded.LLMAPIKey, loaded.NVDAPIKey)
	}
	if loaded.TimezoneOffsetMinutes != value.TimezoneOffsetMinutes {
		t.Fatalf("timezone offset = %d, want %d", loaded.TimezoneOffsetMinutes, value.TimezoneOffsetMinutes)
	}
	var rawLLM, rawNVD string
	if err := db.Raw(`SELECT llm_api_key, nvd_api_key FROM settings WHERE id = 1`).Row().Scan(&rawLLM, &rawNVD); err != nil {
		t.Fatalf("read raw settings: %v", err)
	}
	if strings.Contains(rawLLM, "llm-secret") || strings.Contains(rawNVD, "nvd-secret") {
		t.Fatal("plaintext credential found in database")
	}
}

func TestRepositoryPreservesSecrets_whenSavedValuesAreBlank(t *testing.T) {
	// Given persisted settings containing both supported credentials.
	databasePath := filepath.Join(t.TempDir(), "dashboard.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository, err := NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	value := api.Settings{Language: "ko", Accent: "#4f6ef7", LLMBaseURL: "https://api.openai.com/v1", LLMModel: "gpt-4o-mini", LLMAPIKey: "llm-secret", LLMTimeout: 60, NVDAPIKey: "nvd-secret", TimezoneOffsetMinutes: 540}
	if err := repository.Save(context.Background(), value); err != nil {
		t.Fatalf("save initial settings: %v", err)
	}

	// When a later save leaves both secret fields blank.
	value.LLMAPIKey = ""
	value.NVDAPIKey = "  "
	value.LLMTimeout = 75
	if err := repository.Save(context.Background(), value); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	loaded, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	// Then only the non-secret change is applied.
	if loaded.LLMAPIKey != "llm-secret" || loaded.NVDAPIKey != "nvd-secret" {
		t.Fatalf("loaded secrets = %q / %q, want originals", loaded.LLMAPIKey, loaded.NVDAPIKey)
	}
	if loaded.LLMTimeout != 75 {
		t.Fatalf("timeout = %d, want 75", loaded.LLMTimeout)
	}
}

func TestRepositorySaveWithSourcesRollsBackSource_whenSettingsUpdateFails(t *testing.T) {
	// Given a source change and a database that rejects the settings update.
	databasePath := filepath.Join(t.TempDir(), "dashboard.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	repository, err := NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	value, err := repository.Get(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	value.LLMTimeout = 75
	if err := db.Exec(`CREATE TRIGGER reject_settings_update BEFORE UPDATE ON settings BEGIN SELECT RAISE(FAIL, 'forced settings failure'); END`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	// When settings and a source state are saved together.
	err = repository.SaveWithSources(context.Background(), value, []api.SourceState{{ID: 6, Enabled: false}})

	// Then the settings error rolls back the source state too.
	if err == nil {
		t.Fatal("save succeeded despite the settings failure")
	}
	var source database.Source
	if err := db.First(&source, 6).Error; err != nil {
		t.Fatalf("load source: %v", err)
	}
	if !source.Enabled {
		t.Fatal("source state committed despite the settings failure")
	}
}
