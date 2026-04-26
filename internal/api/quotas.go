// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type QuotasHandler struct {
	db  database.Database
	log *logger.Logger
}

func NewQuotasHandler(log *logger.Logger, db database.Database) *QuotasHandler {
	return &QuotasHandler{db: db, log: log}
}

func (h *QuotasHandler) RegisterRoutes(r chi.Router, pathPrefix string) {
	r.Get(pathPrefix+"/quota", h.handleGetQuota)
	r.Put(pathPrefix+"/quota", h.handleUpdateQuota)
	r.Delete(pathPrefix+"/quota", h.handleDeleteQuota)
}

func (h *QuotasHandler) handleGetQuota(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get quota: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	quota, err := h.db.GetOrCreateStorageQuota(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("get quota: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, quotaToMap(quota))
}

func (h *QuotasHandler) handleUpdateQuota(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("update quota: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	var body struct {
		MaxSizeBytes         *int64 `json:"max_size_bytes"`
		MaxArtifactCount     *int   `json:"max_artifact_count"`
		WarnThresholdPercent *int   `json:"warn_threshold_percent"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	quota, err := h.db.GetOrCreateStorageQuota(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("update quota: db error getting quota", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if body.MaxSizeBytes != nil {
		quota.MaxSizeBytes = *body.MaxSizeBytes
	}

	if body.MaxArtifactCount != nil {
		quota.MaxArtifactCount = *body.MaxArtifactCount
	}

	if body.WarnThresholdPercent != nil {
		quota.WarnThresholdPercent = *body.WarnThresholdPercent
	}

	if err := h.db.UpdateStorageQuota(ctx, quota); err != nil {
		h.log.Errorw("update quota: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, quotaToMap(quota))
}

func (h *QuotasHandler) handleDeleteQuota(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete quota: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	quota, err := h.db.GetOrCreateStorageQuota(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("delete quota: db error getting quota", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Reset to unlimited
	quota.MaxSizeBytes = 0
	quota.MaxArtifactCount = 0

	if err := h.db.UpdateStorageQuota(ctx, quota); err != nil {
		h.log.Errorw("delete quota: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func quotaToMap(q *models.StorageQuota) responses.StorageQuotaResponse {
	return responses.StorageQuotaResponse{
		ID:                   q.ID,
		RepositoryID:         q.RepositoryID,
		MaxSizeBytes:         q.MaxSizeBytes,
		MaxArtifactCount:     q.MaxArtifactCount,
		WarnThresholdPercent: q.WarnThresholdPercent,
		CurrentSizeBytes:     q.CurrentSizeBytes,
		CurrentArtifactCount: q.CurrentArtifactCount,
		UsagePercent:         int(q.UsagePercent()),
		IsExceeded:           q.IsExceeded(),
		CreatedAt:            formatTime(q.CreatedAt),
		UpdatedAt:            formatTime(q.UpdatedAt),
	}
}
