package settings

import (
	"context"
	"database/sql"
	"fmt"

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
    llm_api_key, llm_timeout, nvd_api_key FROM settings WHERE id = 1`).Scan(
		&result.Language, &result.Theme, &result.Accent, &result.LLMBaseURL,
		&result.LLMModel, &llmSecret, &result.LLMTimeout, &nvdSecret)
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
	llmSecret, err := r.secrets.seal(value.LLMAPIKey)
	if err != nil {
		return fmt.Errorf("seal LLM API key: %w", err)
	}
	nvdSecret, err := r.secrets.seal(value.NVDAPIKey)
	if err != nil {
		return fmt.Errorf("seal NVD API key: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE settings SET lang = ?, theme = ?, accent = ?,
    llm_base_url = ?, llm_model = ?, llm_api_key = ?, llm_timeout = ?, nvd_api_key = ? WHERE id = 1`,
		value.Language, value.Theme, value.Accent, value.LLMBaseURL, value.LLMModel,
		llmSecret, value.LLMTimeout, nvdSecret)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}
