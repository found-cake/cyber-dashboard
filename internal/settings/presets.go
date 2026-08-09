package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/database"
	"gorm.io/gorm"
)

var ErrPresetNotFound = errors.New("LLM preset not found")
var ErrBuiltinPreset = errors.New("built-in LLM presets cannot be deleted")
var ErrDuplicatePreset = errors.New("an LLM preset for this endpoint and model already exists")

func (r *Repository) Presets(ctx context.Context) ([]api.LLMPreset, error) {
	var stored []database.LLMPreset
	if err := r.db.WithContext(ctx).Order("builtin DESC, id").Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("query LLM presets: %w", err)
	}
	presets := make([]api.LLMPreset, 0, len(stored))
	for _, item := range stored {
		apiKey, err := r.secrets.open(item.APIKey)
		if err != nil {
			return nil, fmt.Errorf("open LLM preset %d API key: %w", item.ID, err)
		}
		presets = append(presets, api.LLMPreset{ID: item.ID, Label: item.Label, BaseURL: item.BaseURL,
			Model: item.Model, APIKey: apiKey, Builtin: item.Builtin})
	}
	return presets, nil
}

func (r *Repository) CreatePreset(ctx context.Context, request api.CreateLLMPresetRequest) (api.LLMPreset, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(request.BaseURL), "/")
	model := strings.TrimSpace(request.Model)
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || model == "" {
		return api.LLMPreset{}, fmt.Errorf("invalid LLM preset")
	}
	encryptedKey, err := r.secrets.seal(request.APIKey)
	if err != nil {
		return api.LLMPreset{}, fmt.Errorf("seal LLM preset API key: %w", err)
	}
	stored := database.LLMPreset{Label: parsed.Host, BaseURL: baseURL, Model: model, APIKey: encryptedKey}
	if err := r.db.WithContext(ctx).Create(&stored).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return api.LLMPreset{}, ErrDuplicatePreset
		}
		return api.LLMPreset{}, fmt.Errorf("insert LLM preset: %w", err)
	}
	return api.LLMPreset{ID: stored.ID, Label: parsed.Host, BaseURL: baseURL, Model: model, APIKey: request.APIKey}, nil
}

func (r *Repository) UpdatePresetAPIKey(ctx context.Context, id int64, apiKey string) error {
	encryptedKey, replaceKey, err := r.sealReplacement(apiKey)
	if err != nil {
		return fmt.Errorf("seal LLM preset API key: %w", err)
	}
	if !replaceKey {
		var preset database.LLMPreset
		err := r.db.WithContext(ctx).Select("id").First(&preset, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPresetNotFound
		}
		if err != nil {
			return fmt.Errorf("query LLM preset %d: %w", id, err)
		}
		return nil
	}
	result := r.db.WithContext(ctx).Model(&database.LLMPreset{}).Where("id = ?", id).Update("api_key", encryptedKey)
	if result.Error != nil {
		return fmt.Errorf("update LLM preset %d API key: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPresetNotFound
	}
	return nil
}

func (r *Repository) DeletePreset(ctx context.Context, id int64) error {
	var preset database.LLMPreset
	err := r.db.WithContext(ctx).Select("id", "builtin").First(&preset, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPresetNotFound
	}
	if err != nil {
		return fmt.Errorf("query LLM preset %d: %w", id, err)
	}
	if preset.Builtin {
		return ErrBuiltinPreset
	}
	if err := r.db.WithContext(ctx).Delete(&preset).Error; err != nil {
		return fmt.Errorf("delete LLM preset %d: %w", id, err)
	}
	return nil
}
