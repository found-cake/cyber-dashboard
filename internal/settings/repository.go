package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

type Repository struct {
	db      *gorm.DB
	secrets *secretBox
}

func NewRepository(db *gorm.DB, keyPath string) (*Repository, error) {
	secrets, err := openSecretBox(keyPath)
	if err != nil {
		return nil, err
	}
	return &Repository{db: db, secrets: secrets}, nil
}

func (r *Repository) Get(ctx context.Context) (api.Settings, error) {
	var stored database.Setting
	if err := r.db.WithContext(ctx).First(&stored, 1).Error; err != nil {
		return api.Settings{}, fmt.Errorf("query settings: %w", err)
	}
	result := api.Settings{Language: stored.Lang, Accent: stored.Accent, LLMBaseURL: stored.LLMBaseURL,
		LLMModel: stored.LLMModel, LLMTimeout: stored.LLMTimeout}
	if stored.TimezoneOffsetMinutes != nil {
		result.TimezoneOffsetMinutes = *stored.TimezoneOffsetMinutes
	}
	var err error
	result.LLMAPIKey, err = r.secrets.open(stored.LLMAPIKey)
	if err != nil {
		return api.Settings{}, fmt.Errorf("open LLM API key: %w", err)
	}
	result.NVDAPIKey, err = r.secrets.open(stored.NVDAPIKey)
	if err != nil {
		return api.Settings{}, fmt.Errorf("open NVD API key: %w", err)
	}
	return result, nil
}

func (r *Repository) Save(ctx context.Context, value api.Settings) error {
	llmSecret, replaceLLM, err := r.sealReplacement(value.LLMAPIKey)
	if err != nil {
		return fmt.Errorf("seal LLM API key: %w", err)
	}
	nvdSecret, replaceNVD, err := r.sealReplacement(value.NVDAPIKey)
	if err != nil {
		return fmt.Errorf("seal NVD API key: %w", err)
	}
	updates := map[string]any{"lang": value.Language, "accent": value.Accent,
		"llm_base_url": value.LLMBaseURL, "llm_model": value.LLMModel, "llm_timeout": value.LLMTimeout,
		"timezone_offset_minutes": value.TimezoneOffsetMinutes}
	if replaceLLM {
		updates["llm_api_key"] = llmSecret
	}
	if replaceNVD {
		updates["nvd_api_key"] = nvdSecret
	}
	if err := r.db.WithContext(ctx).Model(&database.Setting{}).Where("id = 1").Updates(updates).Error; err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

func (r *Repository) ResolveSecrets(ctx context.Context, value api.Settings) (api.Settings, error) {
	if strings.TrimSpace(value.LLMAPIKey) != "" && strings.TrimSpace(value.NVDAPIKey) != "" {
		return value, nil
	}
	current, err := r.Get(ctx)
	if err != nil {
		return api.Settings{}, err
	}
	if strings.TrimSpace(value.NVDAPIKey) == "" {
		value.NVDAPIKey = current.NVDAPIKey
	}
	if strings.TrimSpace(value.LLMAPIKey) != "" {
		return value, nil
	}
	var preset database.LLMPreset
	err = r.db.WithContext(ctx).Where("base_url = ? AND model = ?",
		strings.TrimRight(strings.TrimSpace(value.LLMBaseURL), "/"), strings.TrimSpace(value.LLMModel)).First(&preset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		value.LLMAPIKey = current.LLMAPIKey
		return value, nil
	}
	if err != nil {
		return api.Settings{}, fmt.Errorf("query matching LLM preset: %w", err)
	}
	presetKey, err := r.secrets.open(preset.APIKey)
	if err != nil {
		return api.Settings{}, fmt.Errorf("open matching LLM preset API key: %w", err)
	}
	if strings.TrimSpace(presetKey) == "" {
		presetKey = current.LLMAPIKey
	}
	value.LLMAPIKey = presetKey
	return value, nil
}

func (r *Repository) sealReplacement(value string) (string, bool, error) {
	if strings.TrimSpace(value) == "" {
		return "", false, nil
	}
	sealed, err := r.secrets.seal(value)
	return sealed, true, err
}
