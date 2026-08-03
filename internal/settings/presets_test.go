package settings

import (
	"context"
	"errors"
	"path/filepath"
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
		BaseURL: "http://localhost:11434/v1/", Model: "qwen3:8b",
	})
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}

	// Then its label is derived from the host and only it can be deleted.
	if created.Label != "localhost:11434" || created.Builtin {
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
