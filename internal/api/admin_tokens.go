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

type AdminTokensHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewAdminTokensHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *AdminTokensHandler {
	return &AdminTokensHandler{db: db, log: log, evaluator: evaluator}
}

func (h *AdminTokensHandler) Register(r chi.Router) {
	r.Post("/api/v1/users/{id}/tokens", h.handleCreate)
	r.Get("/api/v1/users/{id}/tokens", h.handleList)
	r.Delete("/api/v1/users/{id}/tokens/{tokenId}", h.handleRevoke)
	r.Get("/api/v1/admin/tokens", h.handleAdminList)
	r.Delete("/api/v1/admin/tokens/{tokenId}", h.handleAdminRevoke)
}

func (h *AdminTokensHandler) checkOwnerOrAdmin(r *http.Request, targetUserID string) (bool, error) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous {
		return false, nil
	}

	if p.UserID != nil && *p.UserID == targetUserID {
		return true, nil
	}

	return principalIsAdmin(r.Context(), p, h.evaluator)
}

func (h *AdminTokensHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	targetUserID := urlparams.Vars(r)["id"]
	ctx := r.Context()

	ok, err := h.checkOwnerOrAdmin(r, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return
	}

	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return
	}

	var body struct {
		Name      string     `json:"name"`
		Kind      string     `json:"kind"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Kind != auth.TokenKindAccess && body.Kind != auth.TokenKindDeploy {
		writeError(w, http.StatusBadRequest, "validation_failed", "kind must be 'access' or 'deploy'")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return
	}

	if len(body.Scopes) > 0 && !isAdmin {
		callerTargets, err := h.db.GetPermissionTargetsForPrincipal(ctx, *p.UserID, p.GroupIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")
			return
		}

		allowed := make(map[string]bool, len(callerTargets))
		for _, t := range callerTargets {
			allowed[t.ID] = true
		}

		for _, scopeID := range body.Scopes {
			if !allowed[scopeID] {
				writeError(w, http.StatusForbidden, "forbidden", "requested scope not in caller's permissions")
				return
			}
		}
	}

	tokenString, record, err := auth.CreateToken(ctx, h.db, body.Kind, body.Name, targetUserID, body.Scopes, body.ExpiresAt)
	if err != nil {
		h.log.Errorw("create token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "token creation failed")

		return
	}

	writeJSON(w, http.StatusCreated, responses.TokenCreateResponse{
		ID:        record.ID,
		Name:      record.Name,
		Kind:      record.Kind,
		Scopes:    record.Scopes,
		ExpiresAt: formatTimePtr(record.ExpiresAt),
		CreatedAt: formatTime(record.CreatedAt),
		Token:     tokenString,
	})
}

func (h *AdminTokensHandler) handleList(w http.ResponseWriter, r *http.Request) {
	targetUserID := urlparams.Vars(r)["id"]

	ok, err := h.checkOwnerOrAdmin(r, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return
	}

	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return
	}

	tokens, err := h.db.ListTokensByUser(r.Context(), targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	data := make([]any, 0, len(tokens))
	for _, t := range tokens {
		if t.RevokedAt == nil {
			data = append(data, tokenToResponse(t))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *AdminTokensHandler) handleRevoke(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	targetUserID := vars["id"]
	tokenID := vars["tokenId"]
	ctx := r.Context()

	ok, err := h.checkOwnerOrAdmin(r, targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return
	}

	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return
	}

	token, err := h.db.GetTokenByID(ctx, tokenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	if token == nil {
		writeError(w, http.StatusNotFound, "not_found", "token not found")
		return
	}

	if token.UserID != targetUserID {
		writeError(w, http.StatusForbidden, "forbidden", "token does not belong to specified user")
		return
	}

	if err := h.db.RevokeToken(ctx, tokenID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeNoContent(w)
}

func (h *AdminTokensHandler) handleAdminList(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return
	}

	if !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	page := queryIntParam(r, "page", 1, 1, 10000)
	perPage := queryIntParam(r, "per_page", 25, 1, 100)

	tokens, total, err := h.db.ListAllTokens(r.Context(), page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	data := make([]any, 0, len(tokens))
	for _, t := range tokens {
		data = append(data, responses.AdminTokenResponse{
			TokenResponse: tokenToResponse(t),
			UserID:        t.UserID,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":     data,
		"page":     page,
		"per_page": perPage,
		"total":    total,
	})
}

func (h *AdminTokensHandler) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return
	}

	if !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	tokenID := urlparams.Vars(r)["tokenId"]

	token, err := h.db.GetTokenByID(r.Context(), tokenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	if token == nil {
		writeError(w, http.StatusNotFound, "not_found", "token not found")
		return
	}

	if err := h.db.RevokeToken(r.Context(), tokenID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeNoContent(w)
}

func tokenToResponse(t *models.Token) responses.TokenResponse {
	return responses.TokenResponse{
		ID:         t.ID,
		Name:       t.Name,
		Kind:       t.Kind,
		Scopes:     t.Scopes,
		ExpiresAt:  formatTimePtr(t.ExpiresAt),
		LastUsedAt: formatTimePtr(t.LastUsedAt),
		CreatedAt:  formatTime(t.CreatedAt),
	}
}
