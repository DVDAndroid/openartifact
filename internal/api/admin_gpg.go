// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/pkg/logger"
)

type AdminGPGHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewAdminGPGHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *AdminGPGHandler {
	return &AdminGPGHandler{db: db, log: log, evaluator: evaluator}
}

func (h *AdminGPGHandler) Register(r chi.Router) {
	r.Get("/api/v1/admin/gpg/keys", h.handleList)
	r.Post("/api/v1/admin/gpg/keys", h.handleImport)
	r.Post("/api/v1/admin/gpg/keys/generate", h.handleGenerate)
	r.Delete("/api/v1/admin/gpg/keys/{id}", h.handleDelete)
}

func (h *AdminGPGHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
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

func (h *AdminGPGHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"keys": []any{}})
}

func (h *AdminGPGHandler) handleImport(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "GPG key import is not yet available")
}

func (h *AdminGPGHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "GPG key generation is not yet available")
}

func (h *AdminGPGHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	writeError(w, http.StatusNotImplemented, "not_implemented", "GPG key deletion is not yet available")
}
