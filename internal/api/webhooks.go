// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/webhook"
	"github.com/openartifact/openartifact/pkg/logger"
)

type WebhooksHandler struct {
	db         database.Database
	log        *logger.Logger
	evaluator  *auth.Evaluator
	dispatcher *webhook.Dispatcher
}

func NewWebhooksHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator, dispatcher *webhook.Dispatcher) *WebhooksHandler {
	return &WebhooksHandler{
		db:         db,
		log:        log,
		evaluator:  evaluator,
		dispatcher: dispatcher,
	}
}

func (h *WebhooksHandler) RegisterAuthed(r chi.Router) {
	// Global webhooks
	r.Get("/api/v1/webhooks", h.handleListGlobal)
	r.Post("/api/v1/webhooks", h.handleCreateGlobal)
	r.Get("/api/v1/webhooks/{id}", h.handleGet)
	r.Put("/api/v1/webhooks/{id}", h.handleUpdate)
	r.Delete("/api/v1/webhooks/{id}", h.handleDelete)
	r.Get("/api/v1/webhooks/{id}/deliveries", h.handleListDeliveries)
	r.Post("/api/v1/webhooks/{id}/test", h.handleTest)

	// Repository webhooks
	r.Get("/api/v1/repositories/{name}/webhooks", h.handleListRepo)
	r.Post("/api/v1/repositories/{name}/webhooks", h.handleCreateRepo)
}

// handleListGlobal lists all global webhooks.
func (h *WebhooksHandler) handleListGlobal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	webhooks, err := h.db.ListWebhooks(ctx, "")
	if err != nil {
		h.log.Errorw("list webhooks: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"webhooks": webhooks,
	})
}

// handleCreateGlobal creates a new global webhook.
func (h *WebhooksHandler) handleCreateGlobal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string   `json:"name"`
		URL            string   `json:"url"`
		Secret         string   `json:"secret,omitempty"`
		Events         []string `json:"events"`
		ContentType    string   `json:"content_type,omitempty"`
		SSLVerify      bool     `json:"ssl_verify,omitempty"`
		MaxRetries     int      `json:"max_retries,omitempty"`
		TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return
	}

	if req.Name == "" || req.URL == "" || len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "name, url, and events are required")
		return
	}

	if req.ContentType == "" {
		req.ContentType = "application/json"
	}

	if !req.SSLVerify {
		req.SSLVerify = true
	}

	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}

	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 30
	}

	wh := &models.Webhook{
		Name:           req.Name,
		URL:            req.URL,
		SecretEnc:      req.Secret,
		Enabled:        true,
		Events:         req.Events,
		ContentType:    req.ContentType,
		SSLVerify:      req.SSLVerify,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	if err := h.db.CreateWebhook(r.Context(), wh); err != nil {
		h.log.Errorw("create webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, wh)
}

// handleListRepo lists all webhooks for a repository.
func (h *WebhooksHandler) handleListRepo(w http.ResponseWriter, r *http.Request) {
	repoName := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, repoName)
	if err != nil {
		h.log.Errorw("get repository: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	webhooks, err := h.db.ListWebhooks(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("list webhooks: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"webhooks": webhooks,
	})
}

// handleCreateRepo creates a new webhook for a repository.
func (h *WebhooksHandler) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	repoName := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, repoName)
	if err != nil {
		h.log.Errorw("get repository: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	var req struct {
		Name           string   `json:"name"`
		URL            string   `json:"url"`
		Secret         string   `json:"secret,omitempty"`
		Events         []string `json:"events"`
		ContentType    string   `json:"content_type,omitempty"`
		SSLVerify      bool     `json:"ssl_verify,omitempty"`
		MaxRetries     int      `json:"max_retries,omitempty"`
		TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return
	}

	if req.Name == "" || req.URL == "" || len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "name, url, and events are required")
		return
	}

	if req.ContentType == "" {
		req.ContentType = "application/json"
	}

	if !req.SSLVerify {
		req.SSLVerify = true
	}

	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}

	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 30
	}

	wh := &models.Webhook{
		RepositoryID:   repo.ID,
		Name:           req.Name,
		URL:            req.URL,
		SecretEnc:      req.Secret,
		Enabled:        true,
		Events:         req.Events,
		ContentType:    req.ContentType,
		SSLVerify:      req.SSLVerify,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	if err := h.db.CreateWebhook(ctx, wh); err != nil {
		h.log.Errorw("create webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, wh)
}

// handleGet retrieves a webhook by ID.
func (h *WebhooksHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	wh, err := h.db.GetWebhook(ctx, id)
	if err != nil {
		h.log.Errorw("get webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if wh == nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	writeJSON(w, http.StatusOK, wh)
}

// handleUpdate updates a webhook.
func (h *WebhooksHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	wh, err := h.db.GetWebhook(ctx, id)
	if err != nil {
		h.log.Errorw("get webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if wh == nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	var req struct {
		Name           string   `json:"name,omitempty"`
		URL            string   `json:"url,omitempty"`
		Secret         string   `json:"secret,omitempty"`
		Enabled        *bool    `json:"enabled,omitempty"`
		Events         []string `json:"events,omitempty"`
		ContentType    string   `json:"content_type,omitempty"`
		SSLVerify      *bool    `json:"ssl_verify,omitempty"`
		MaxRetries     *int     `json:"max_retries,omitempty"`
		TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return
	}

	if req.Name != "" {
		wh.Name = req.Name
	}

	if req.URL != "" {
		wh.URL = req.URL
	}

	if req.Secret != "" {
		wh.SecretEnc = req.Secret
	}

	if req.Enabled != nil {
		wh.Enabled = *req.Enabled
	}

	if len(req.Events) > 0 {
		wh.Events = req.Events
	}

	if req.ContentType != "" {
		wh.ContentType = req.ContentType
	}

	if req.SSLVerify != nil {
		wh.SSLVerify = *req.SSLVerify
	}

	if req.MaxRetries != nil {
		wh.MaxRetries = *req.MaxRetries
	}

	if req.TimeoutSeconds != nil {
		wh.TimeoutSeconds = *req.TimeoutSeconds
	}

	if err := h.db.UpdateWebhook(ctx, wh); err != nil {
		h.log.Errorw("update webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, wh)
}

// handleDelete deletes a webhook.
func (h *WebhooksHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	wh, err := h.db.GetWebhook(ctx, id)
	if err != nil {
		h.log.Errorw("get webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if wh == nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.db.DeleteWebhook(ctx, id); err != nil {
		h.log.Errorw("delete webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListDeliveries lists deliveries for a webhook.
func (h *WebhooksHandler) handleListDeliveries(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	wh, err := h.db.GetWebhook(ctx, id)
	if err != nil {
		h.log.Errorw("get webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if wh == nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	limit := parseIntClamp(r.URL.Query().Get("limit"), 50, 1, 500)

	deliveries, err := h.db.ListWebhookDeliveries(ctx, id, limit)
	if err != nil {
		h.log.Errorw("list webhook deliveries: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deliveries": deliveries,
	})
}

// handleTest sends a test event to a webhook.
func (h *WebhooksHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	wh, err := h.db.GetWebhook(ctx, id)
	if err != nil {
		h.log.Errorw("get webhook: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if wh == nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	// Create a test payload
	testPayload := map[string]any{
		"event":     "webhook.test",
		"message":   "This is a test webhook delivery",
		"timestamp": time.Now().Unix(),
	}

	// Dispatch the test event
	h.dispatcher.Dispatch(ctx, wh.RepositoryID, "webhook.test", testPayload)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"message": "test event queued",
	})
}
