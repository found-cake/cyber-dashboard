package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
)

type Repository struct {
	db      *sql.DB
	secrets *secretBox
}

func NewRepository(db *sql.DB, keyPath string) (*Repository, error) {
	secrets, err := openSecretBox(keyPath)
	if err != nil {
		return nil, err
	}
	return &Repository{db: db, secrets: secrets}, nil
}

func (r *Repository) Get(ctx context.Context) (api.Settings, error) {
	var result api.Settings
	var llmSecret, nvdSecret string
	err := r.db.QueryRowContext(ctx, `SELECT lang, theme, accent, llm_base_url, llm_model,
    llm_api_key, llm_timeout, nvd_api_key, timezone_offset_minutes FROM settings WHERE id = 1`).Scan(
		&result.Language, &result.Theme, &result.Accent, &result.LLMBaseURL,
		&result.LLMModel, &llmSecret, &result.LLMTimeout, &nvdSecret, &result.TimezoneOffsetMinutes)
	if err != nil {
		return api.Settings{}, fmt.Errorf("query settings: %w", err)
	}
	result.LLMAPIKey, err = r.secrets.open(llmSecret)
	if err != nil {
		return api.Settings{}, fmt.Errorf("open LLM API key: %w", err)
	}
	result.NVDAPIKey, err = r.secrets.open(nvdSecret)
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
	_, err = r.db.ExecContext(ctx, `UPDATE settings SET lang = ?, theme = ?, accent = ?,
	llm_base_url = ?, llm_model = ?, llm_api_key = CASE WHEN ? THEN ? ELSE llm_api_key END,
	llm_timeout = ?, nvd_api_key = CASE WHEN ? THEN ? ELSE nvd_api_key END, timezone_offset_minutes = ? WHERE id = 1`,
		value.Language, value.Theme, value.Accent, value.LLMBaseURL, value.LLMModel,
		replaceLLM, llmSecret, value.LLMTimeout, replaceNVD, nvdSecret, value.TimezoneOffsetMinutes)
	if err != nil {
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
	var encryptedKey string
	err = r.db.QueryRowContext(ctx, `SELECT api_key FROM llm_presets WHERE base_url = ? AND model = ?`,
		strings.TrimRight(strings.TrimSpace(value.LLMBaseURL), "/"), strings.TrimSpace(value.LLMModel)).Scan(&encryptedKey)
	if errors.Is(err, sql.ErrNoRows) {
		value.LLMAPIKey = current.LLMAPIKey
		return value, nil
	}
	if err != nil {
		return api.Settings{}, fmt.Errorf("query matching LLM preset: %w", err)
	}
	presetKey, err := r.secrets.open(encryptedKey)
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
