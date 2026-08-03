package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/found-cake/cyber-dashboard/api"
)

var ErrPresetNotFound = errors.New("LLM preset not found")
var ErrBuiltinPreset = errors.New("built-in LLM presets cannot be deleted")
var ErrDuplicatePreset = errors.New("an LLM preset for this endpoint and model already exists")

func (r *Repository) Presets(ctx context.Context) ([]api.LLMPreset, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, label, base_url, model, builtin
    FROM llm_presets ORDER BY builtin DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("query LLM presets: %w", err)
	}
	defer rows.Close()
	presets := []api.LLMPreset{}
	for rows.Next() {
		var preset api.LLMPreset
		if err := rows.Scan(&preset.ID, &preset.Label, &preset.BaseURL, &preset.Model, &preset.Builtin); err != nil {
			return nil, fmt.Errorf("scan LLM preset: %w", err)
		}
		presets = append(presets, preset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate LLM presets: %w", err)
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
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM llm_presets WHERE base_url = ? AND model = ?`, baseURL, model).Scan(&exists); err != nil {
		return api.LLMPreset{}, fmt.Errorf("check LLM preset: %w", err)
	}
	if exists > 0 {
		return api.LLMPreset{}, ErrDuplicatePreset
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO llm_presets (label, base_url, model, builtin)
    VALUES (?, ?, ?, 0)`, parsed.Host, baseURL, model)
	if err != nil {
		return api.LLMPreset{}, fmt.Errorf("insert LLM preset: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return api.LLMPreset{}, fmt.Errorf("LLM preset id: %w", err)
	}
	return api.LLMPreset{ID: id, Label: parsed.Host, BaseURL: baseURL, Model: model}, nil
}

func (r *Repository) DeletePreset(ctx context.Context, id int64) error {
	var builtin bool
	err := r.db.QueryRowContext(ctx, `SELECT builtin FROM llm_presets WHERE id = ?`, id).Scan(&builtin)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPresetNotFound
	}
	if err != nil {
		return fmt.Errorf("query LLM preset %d: %w", id, err)
	}
	if builtin {
		return ErrBuiltinPreset
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM llm_presets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete LLM preset %d: %w", id, err)
	}
	return nil
}
