// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type PropertiesHandler struct {
	db  database.Database
	log *logger.Logger
}

func NewPropertiesHandler(log *logger.Logger, db database.Database) *PropertiesHandler {
	return &PropertiesHandler{db: db, log: log}
}

func (h *PropertiesHandler) RegisterRoutes(r chi.Router, pathPrefix string) {
	r.Get(pathPrefix+"/artifacts/{artifact_id}/properties", h.handleGetProperties)
	r.Put(pathPrefix+"/artifacts/{artifact_id}/properties", h.handleSetProperties)
	r.Patch(pathPrefix+"/artifacts/{artifact_id}/properties", h.handlePatchProperties)
	r.Delete(pathPrefix+"/artifacts/{artifact_id}/properties/{key}", h.handleDeleteProperty)
	r.Get(pathPrefix+"/artifacts/search", h.handleSearchByProperty)
}

func (h *PropertiesHandler) handleGetProperties(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	artifactIDEncoded := urlparams.Vars(r)["artifact_id"]
	ctx := r.Context()

	// Decode the artifact ID from URL encoding
	artifactID, err := url.PathUnescape(artifactIDEncoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid artifact_id")
		return
	}

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get properties: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	props, err := h.db.ListArtifactProperties(ctx, repo.ID, artifactID)
	if err != nil {
		h.log.Errorw("get properties: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if props == nil {
		props = make(map[string]string)
	}

	writeJSON(w, http.StatusOK, props)
}

func (h *PropertiesHandler) handleSetProperties(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	artifactIDEncoded := urlparams.Vars(r)["artifact_id"]
	ctx := r.Context()

	// Decode the artifact ID from URL encoding
	artifactID, err := url.PathUnescape(artifactIDEncoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid artifact_id")
		return
	}

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("set properties: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	var body map[string]string
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Get existing properties to delete them
	existingProps, err := h.db.ListArtifactProperties(ctx, repo.ID, artifactID)
	if err != nil {
		h.log.Errorw("set properties: db error getting existing properties", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Delete all existing properties
	for key := range existingProps {
		if err := h.db.DeleteArtifactProperty(ctx, repo.ID, artifactID, key); err != nil {
			h.log.Errorw("set properties: db error deleting property", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	// Set new properties
	for key, value := range body {
		if err := h.db.SetArtifactProperty(ctx, repo.ID, artifactID, key, value); err != nil {
			h.log.Errorw("set properties: db error setting property", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	writeJSON(w, http.StatusOK, body)
}

func (h *PropertiesHandler) handlePatchProperties(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	artifactIDEncoded := urlparams.Vars(r)["artifact_id"]
	ctx := r.Context()

	// Decode the artifact ID from URL encoding
	artifactID, err := url.PathUnescape(artifactIDEncoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid artifact_id")
		return
	}

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("patch properties: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	var body map[string]string
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Upsert each property
	for key, value := range body {
		if err := h.db.SetArtifactProperty(ctx, repo.ID, artifactID, key, value); err != nil {
			h.log.Errorw("patch properties: db error setting property", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	// Return all properties
	props, err := h.db.ListArtifactProperties(ctx, repo.ID, artifactID)
	if err != nil {
		h.log.Errorw("patch properties: db error getting properties", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if props == nil {
		props = make(map[string]string)
	}

	writeJSON(w, http.StatusOK, props)
}

func (h *PropertiesHandler) handleDeleteProperty(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	artifactIDEncoded := urlparams.Vars(r)["artifact_id"]
	key := urlparams.Vars(r)["key"]
	ctx := r.Context()

	// Decode the artifact ID from URL encoding
	artifactID, err := url.PathUnescape(artifactIDEncoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid artifact_id")
		return
	}

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete property: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	if err := h.db.DeleteArtifactProperty(ctx, repo.ID, artifactID, key); err != nil {
		h.log.Errorw("delete property: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *PropertiesHandler) handleSearchByProperty(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")

	if key == "" || value == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "key and value query parameters are required")
		return
	}

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("search by property: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	artifactIDs, err := h.db.SearchByProperty(ctx, repo.ID, key, value)
	if err != nil {
		h.log.Errorw("search by property: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if artifactIDs == nil {
		artifactIDs = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"artifact_ids": artifactIDs,
	})
}
