// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

//nolint:gochecknoglobals // immutable lookup table
var validActions = map[string]bool{
	"read": true, "write": true, "delete": true, "manage": true, "annotate": true,
}

type AdminPermissionsHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewAdminPermissionsHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *AdminPermissionsHandler {
	return &AdminPermissionsHandler{db: db, log: log, evaluator: evaluator}
}

func (h *AdminPermissionsHandler) Register(r chi.Router) {
	r.Post("/api/v1/permission-targets", h.handleCreate)
	r.Get("/api/v1/permission-targets", h.handleList)
	r.Get("/api/v1/permission-targets/{id}", h.handleGet)
	r.Patch("/api/v1/permission-targets/{id}", h.handlePatch)
	r.Delete("/api/v1/permission-targets/{id}", h.handleDelete)
	r.Post("/api/v1/permission-targets/{id}/assignments", h.handleAssign)
	r.Delete("/api/v1/permission-targets/{id}/assignments", h.handleUnassign)
}

func (h *AdminPermissionsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name             string   `json:"name"`
		RepositoryFilter string   `json:"repository_filter"`
		PathPattern      string   `json:"path_pattern"`
		Actions          []string `json:"actions"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if _, err := regexp.Compile(body.RepositoryFilter); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid repository_filter regex")
		return
	}

	if _, err := regexp.Compile(body.PathPattern); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid path_pattern regex")
		return
	}

	if err := validateActions(body.Actions); err != "" {
		writeError(w, http.StatusBadRequest, "validation_failed", err)
		return
	}

	now := time.Now()
	target := &models.PermissionTarget{
		Name:             body.Name,
		RepositoryFilter: body.RepositoryFilter,
		PathPattern:      body.PathPattern,
		Actions:          body.Actions,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	saved, err := h.db.CreatePermissionTarget(r.Context(), target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeJSON(w, http.StatusCreated, permTargetToResponse(saved))
}

func (h *AdminPermissionsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	targets, err := h.db.ListPermissionTargets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	data := make([]any, 0, len(targets))
	for _, t := range targets {
		data = append(data, permTargetToResponse(t))
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *AdminPermissionsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]

	target, err := h.db.GetPermissionTargetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	if target == nil {
		writeError(w, http.StatusNotFound, "not_found", "permission target not found")
		return
	}

	writeJSON(w, http.StatusOK, permTargetToResponse(target))
}

func (h *AdminPermissionsHandler) handlePatch(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	target, err := h.db.GetPermissionTargetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	if target == nil {
		writeError(w, http.StatusNotFound, "not_found", "permission target not found")
		return
	}

	var body struct {
		Name             *string  `json:"name"`
		RepositoryFilter *string  `json:"repository_filter"`
		PathPattern      *string  `json:"path_pattern"`
		Actions          []string `json:"actions"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.RepositoryFilter != nil {
		if _, err := regexp.Compile(*body.RepositoryFilter); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid repository_filter regex")
			return
		}

		target.RepositoryFilter = *body.RepositoryFilter
	}

	if body.PathPattern != nil {
		if _, err := regexp.Compile(*body.PathPattern); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid path_pattern regex")
			return
		}

		target.PathPattern = *body.PathPattern
	}

	if body.Actions != nil {
		if msg := validateActions(body.Actions); msg != "" {
			writeError(w, http.StatusBadRequest, "validation_failed", msg)
			return
		}

		target.Actions = body.Actions
	}

	if body.Name != nil {
		target.Name = *body.Name
	}

	target.UpdatedAt = time.Now()
	if err := h.db.UpdatePermissionTarget(ctx, target); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeJSON(w, http.StatusOK, permTargetToResponse(target))
}

func (h *AdminPermissionsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	if err := h.db.DeletePermissionTarget(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeNoContent(w)
}

func (h *AdminPermissionsHandler) handleAssign(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]

	var body struct {
		PrincipalType string `json:"principal_type"`
		PrincipalID   string `json:"principal_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.PrincipalType != "user" && body.PrincipalType != "group" {
		writeError(w, http.StatusBadRequest, "validation_failed", "principal_type must be 'user' or 'group'")
		return
	}

	if body.PrincipalID == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "principal_id is required")
		return
	}

	assignment := &models.PermissionAssignment{
		PermissionTargetID: id,
		PrincipalType:      body.PrincipalType,
		PrincipalID:        body.PrincipalID,
	}
	if err := h.db.CreatePermissionAssignment(r.Context(), assignment); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeNoContent(w)
}

func (h *AdminPermissionsHandler) handleUnassign(w http.ResponseWriter, r *http.Request) {
	id := urlparams.Vars(r)["id"]
	principalType := r.URL.Query().Get("principal_type")
	principalID := r.URL.Query().Get("principal_id")

	if principalType != "user" && principalType != "group" {
		writeError(w, http.StatusBadRequest, "validation_failed", "principal_type must be 'user' or 'group'")
		return
	}

	if principalID == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "principal_id is required")
		return
	}

	if err := h.db.DeletePermissionAssignment(r.Context(), id, principalType, principalID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeNoContent(w)
}

func validateActions(actions []string) string {
	for _, a := range actions {
		if !validActions[a] {
			return "invalid action: " + a + "; must be one of read, write, delete, manage, annotate"
		}
	}

	return ""
}

func permTargetToResponse(t *models.PermissionTarget) responses.PermissionTargetResponse {
	return responses.PermissionTargetResponse{
		ID:               t.ID,
		Name:             t.Name,
		RepositoryFilter: t.RepositoryFilter,
		PathPattern:      t.PathPattern,
		Actions:          t.Actions,
		CreatedAt:        formatTime(t.CreatedAt),
		UpdatedAt:        formatTime(t.UpdatedAt),
	}
}
