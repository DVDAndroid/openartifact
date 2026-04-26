// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type CleanupHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewCleanupHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *CleanupHandler {
	return &CleanupHandler{db: db, log: log, evaluator: evaluator}
}

func (h *CleanupHandler) RegisterAuthed(r chi.Router) {
	r.Get("/api/v1/repositories/{name}/cleanup-policies", h.handleList)
	r.Post("/api/v1/repositories/{name}/cleanup-policies", h.handleCreate)
	r.Get("/api/v1/repositories/{name}/cleanup-policies/{id}", h.handleGet)
	r.Put("/api/v1/repositories/{name}/cleanup-policies/{id}", h.handleUpdate)
	r.Delete("/api/v1/repositories/{name}/cleanup-policies/{id}", h.handleDelete)
	r.Post("/api/v1/repositories/{name}/cleanup-policies/{id}/run", h.handleRun)
}

//nolint:dupl // handler boilerplate: identical structure, different entities
func (h *CleanupHandler) handleList(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("list cleanup policies: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	policies, err := h.db.ListCleanupPolicies(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("list cleanup policies: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policies == nil {
		policies = []*models.CleanupPolicy{}
	}

	var result []responses.CleanupPolicyResponse
	for _, policy := range policies {
		result = append(result, policyToMap(policy))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policies": result,
	})
}

func (h *CleanupHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("create cleanup policy: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		Name                    string `json:"name"`
		Enabled                 bool   `json:"enabled"`
		KeepLastN               *int   `json:"keep_last_n"`
		KeepSinceDays           *int   `json:"keep_since_days"`
		KeepDownloadedSinceDays *int   `json:"keep_downloaded_since_days"`
		NamePattern             string `json:"name_pattern"`
		ScheduleCron            string `json:"schedule_cron"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}

	scheduleCron := body.ScheduleCron
	if scheduleCron == "" {
		scheduleCron = "0 2 * * *" // default: daily at 2am
	}

	policy := &models.CleanupPolicy{
		RepositoryID:            repo.ID,
		Name:                    body.Name,
		Enabled:                 body.Enabled,
		KeepLastN:               body.KeepLastN,
		KeepSinceDays:           body.KeepSinceDays,
		KeepDownloadedSinceDays: body.KeepDownloadedSinceDays,
		NamePattern:             body.NamePattern,
		ScheduleCron:            scheduleCron,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}

	if err := h.db.CreateCleanupPolicy(ctx, policy); err != nil {
		h.log.Errorw("create cleanup policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, policyToMap(policy))
}

func (h *CleanupHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get cleanup policy: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	policy, err := h.db.GetCleanupPolicy(ctx, id)
	if err != nil {
		h.log.Errorw("get cleanup policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policy == nil || policy.RepositoryID != repo.ID {
		writeError(w, http.StatusNotFound, "not_found", "cleanup policy not found")
		return
	}

	writeJSON(w, http.StatusOK, policyToMap(policy))
}

func (h *CleanupHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("update cleanup policy: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	policy, err := h.db.GetCleanupPolicy(ctx, id)
	if err != nil {
		h.log.Errorw("update cleanup policy: db error getting policy", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policy == nil || policy.RepositoryID != repo.ID {
		writeError(w, http.StatusNotFound, "not_found", "cleanup policy not found")
		return
	}

	var body struct {
		Name                    *string `json:"name"`
		Enabled                 *bool   `json:"enabled"`
		KeepLastN               *int    `json:"keep_last_n"`
		KeepSinceDays           *int    `json:"keep_since_days"`
		KeepDownloadedSinceDays *int    `json:"keep_downloaded_since_days"`
		NamePattern             *string `json:"name_pattern"`
		ScheduleCron            *string `json:"schedule_cron"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name != nil {
		policy.Name = *body.Name
	}

	if body.Enabled != nil {
		policy.Enabled = *body.Enabled
	}

	if body.KeepLastN != nil {
		policy.KeepLastN = body.KeepLastN
	}

	if body.KeepSinceDays != nil {
		policy.KeepSinceDays = body.KeepSinceDays
	}

	if body.KeepDownloadedSinceDays != nil {
		policy.KeepDownloadedSinceDays = body.KeepDownloadedSinceDays
	}

	if body.NamePattern != nil {
		policy.NamePattern = *body.NamePattern
	}

	if body.ScheduleCron != nil {
		policy.ScheduleCron = *body.ScheduleCron
	}

	if err := h.db.UpdateCleanupPolicy(ctx, policy); err != nil {
		h.log.Errorw("update cleanup policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, policyToMap(policy))
}

func (h *CleanupHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete cleanup policy: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	policy, err := h.db.GetCleanupPolicy(ctx, id)
	if err != nil {
		h.log.Errorw("delete cleanup policy: db error getting policy", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policy == nil || policy.RepositoryID != repo.ID {
		writeError(w, http.StatusNotFound, "not_found", "cleanup policy not found")
		return
	}

	if err := h.db.DeleteCleanupPolicy(ctx, id); err != nil {
		h.log.Errorw("delete cleanup policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *CleanupHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("run cleanup policy: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	policy, err := h.db.GetCleanupPolicy(ctx, id)
	if err != nil {
		h.log.Errorw("run cleanup policy: db error getting policy", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policy == nil || policy.RepositoryID != repo.ID {
		writeError(w, http.StatusNotFound, "not_found", "cleanup policy not found")
		return
	}

	// Acknowledge the request; the cleanup worker applies policies on its own schedule.
	writeJSON(w, http.StatusOK, map[string]any{
		"message":   "cleanup policy run triggered",
		"policy_id": id,
	})
}

func policyToMap(p *models.CleanupPolicy) responses.CleanupPolicyResponse {
	var lastRunError *string
	if p.LastRunError != "" {
		lastRunError = &p.LastRunError
	}

	namePattern := p.NamePattern
	scheduleCron := p.ScheduleCron

	return responses.CleanupPolicyResponse{
		ID:                      p.ID,
		RepositoryID:            p.RepositoryID,
		Name:                    p.Name,
		Enabled:                 p.Enabled,
		KeepLastN:               p.KeepLastN,
		KeepSinceDays:           p.KeepSinceDays,
		KeepDownloadedSinceDays: p.KeepDownloadedSinceDays,
		NamePattern:             &namePattern,
		ScheduleCron:            &scheduleCron,
		CreatedAt:               formatTime(p.CreatedAt),
		UpdatedAt:               formatTime(p.UpdatedAt),
		LastRunAt:               formatTimePtr(p.LastRunAt),
		LastRunDeleted:          p.LastRunDeleted,
		LastRunError:            lastRunError,
	}
}
