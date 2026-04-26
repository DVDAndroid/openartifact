// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/ai"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/pkg/logger"
)

type AdminAIHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewAdminAIHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *AdminAIHandler {
	return &AdminAIHandler{db: db, log: log, evaluator: evaluator}
}

func (h *AdminAIHandler) Register(r chi.Router) {
	r.Get("/api/v1/admin/ai/config", h.handleGetConfig)
	r.Put("/api/v1/admin/ai/config", h.handlePutConfig)
	r.Post("/api/v1/admin/ai/test", h.handleTest)
	r.Get("/api/v1/admin/ai/models", h.handleGetModels)
}

func (h *AdminAIHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return false
	}

	if !isAdmin {
		writeError(w, http.StatusForbidden, "admin_required", "admin access required")
		return false
	}

	return true
}

// loadConfigFromDB reads AI config from system_settings table.
//
//nolint:unparam // error return kept for future use
func (h *AdminAIHandler) loadConfigFromDB(ctx context.Context) (ai.Config, error) {
	cfg := ai.Config{}

	// Read each setting
	// Note: enabled is not stored in ai.Config, it's handled separately

	provider, _ := h.db.GetSystemSetting(ctx, "ai.provider")
	cfg.Provider = provider

	baseURL, _ := h.db.GetSystemSetting(ctx, "ai.base_url")
	cfg.BaseURL = baseURL

	apiKey, _ := h.db.GetSystemSetting(ctx, "ai.api_key")
	cfg.APIKey = apiKey

	model, _ := h.db.GetSystemSetting(ctx, "ai.model")
	cfg.Model = model

	maxTokensStr, _ := h.db.GetSystemSetting(ctx, "ai.max_tokens")
	if maxTokensStr != "" {
		if n, err := strconv.Atoi(maxTokensStr); err == nil {
			cfg.MaxTokens = n
		}
	}

	timeoutSecsStr, _ := h.db.GetSystemSetting(ctx, "ai.timeout_secs")
	if timeoutSecsStr != "" {
		if n, err := strconv.Atoi(timeoutSecsStr); err == nil {
			cfg.TimeoutSecs = n
		}
	}

	systemPrompt, _ := h.db.GetSystemSetting(ctx, "ai.system_prompt")
	cfg.SystemPrompt = systemPrompt

	return cfg, nil
}

func (h *AdminAIHandler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	cfg, err := h.loadConfigFromDB(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to load config")
		return
	}

	enabled, _ := h.db.GetSystemSetting(r.Context(), "ai.enabled")
	enabledBool := false

	if enabled != "" {
		if b, err := strconv.ParseBool(enabled); err == nil {
			enabledBool = b
		}
	}

	// Mask API key
	apiKeyDisplay := ""
	if cfg.APIKey != "" {
		apiKeyDisplay = "••••••••" //nolint:gosec // This is a display mask, not a real credential
	}

	response := map[string]any{
		"enabled":       enabledBool,
		"provider":      cfg.Provider,
		"base_url":      cfg.BaseURL,
		"api_key":       apiKeyDisplay,
		"model":         cfg.Model,
		"max_tokens":    cfg.MaxTokens,
		"timeout_secs":  cfg.TimeoutSecs,
		"system_prompt": cfg.SystemPrompt,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *AdminAIHandler) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var body struct {
		Enabled      bool   `json:"enabled"`
		Provider     string `json:"provider"`
		BaseURL      string `json:"base_url"`
		APIKey       string `json:"api_key"`
		Model        string `json:"model"`
		MaxTokens    int    `json:"max_tokens"`
		TimeoutSecs  int    `json:"timeout_secs"`
		SystemPrompt string `json:"system_prompt"`
	}

	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ctx := r.Context()

	// Save enabled
	if err := h.db.SetSystemSetting(ctx, "ai.enabled", strconv.FormatBool(body.Enabled)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
		return
	}

	// Save provider
	if body.Provider != "" {
		if err := h.db.SetSystemSetting(ctx, "ai.provider", body.Provider); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Save base_url
	if body.BaseURL != "" {
		if err := h.db.SetSystemSetting(ctx, "ai.base_url", body.BaseURL); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Save api_key (only if not masked)
	if body.APIKey != "" && body.APIKey != "••••••••" {
		if err := h.db.SetSystemSetting(ctx, "ai.api_key", body.APIKey); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Save model
	if body.Model != "" {
		if err := h.db.SetSystemSetting(ctx, "ai.model", body.Model); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Save max_tokens
	if body.MaxTokens > 0 {
		if err := h.db.SetSystemSetting(ctx, "ai.max_tokens", strconv.Itoa(body.MaxTokens)); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Save timeout_secs
	if body.TimeoutSecs > 0 {
		if err := h.db.SetSystemSetting(ctx, "ai.timeout_secs", strconv.Itoa(body.TimeoutSecs)); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Save system_prompt
	if body.SystemPrompt != "" {
		if err := h.db.SetSystemSetting(ctx, "ai.system_prompt", body.SystemPrompt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save config")
			return
		}
	}

	// Return the saved config
	h.handleGetConfig(w, r)
}

func (h *AdminAIHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var body *struct {
		Enabled      bool   `json:"enabled"`
		Provider     string `json:"provider"`
		BaseURL      string `json:"base_url"`
		APIKey       string `json:"api_key"`
		Model        string `json:"model"`
		MaxTokens    int    `json:"max_tokens"`
		TimeoutSecs  int    `json:"timeout_secs"`
		SystemPrompt string `json:"system_prompt"`
	}

	// Try to decode body, but it's optional
	_ = decodeJSON(r, &body)

	var cfg ai.Config
	if body != nil && body.Provider != "" {
		// Use provided config
		cfg = ai.Config{
			Provider:     body.Provider,
			BaseURL:      body.BaseURL,
			APIKey:       body.APIKey,
			Model:        body.Model,
			MaxTokens:    body.MaxTokens,
			TimeoutSecs:  body.TimeoutSecs,
			SystemPrompt: body.SystemPrompt,
		}
	} else {
		// Load from DB
		var err error

		cfg, err = h.loadConfigFromDB(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to load config")
			return
		}
	}

	// Create client and test
	client := ai.NewClient(cfg)

	models, err := client.ListModels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"models": models,
	})
}

func (h *AdminAIHandler) handleGetModels(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	cfg, err := h.loadConfigFromDB(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to load config")
		return
	}

	client := ai.NewClient(cfg)

	models, err := client.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"models": models,
	})
}
