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
	t.Cleanup(func() { _ = db.Close() })
	repository, err := NewRepository(db, databasePath+".key")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	value := api.Settings{Language: "ko", Theme: "dark", Accent: "#4f6ef7", LLMBaseURL: "https://api.openai.com/v1", LLMModel: "gpt-4o-mini", LLMAPIKey: "llm-secret", LLMTimeout: 60, NVDAPIKey: "nvd-secret"}

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
	var rawLLM, rawNVD string
	if err := db.QueryRow(`SELECT llm_api_key, nvd_api_key FROM settings WHERE id = 1`).Scan(&rawLLM, &rawNVD); err != nil {
		t.Fatalf("read raw settings: %v", err)
	}
	if strings.Contains(rawLLM, "llm-secret") || strings.Contains(rawNVD, "nvd-secret") {
		t.Fatal("plaintext credential found in database")
	}
}
