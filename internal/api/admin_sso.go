// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/config"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

var slugRe = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

type AdminSSOHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
	baseURL   string
	cfg       *config.Config
}

func NewAdminSSOHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator, baseURL string, cfg *config.Config) *AdminSSOHandler {
	return &AdminSSOHandler{db: db, log: log, evaluator: evaluator, baseURL: baseURL, cfg: cfg}
}

func (h *AdminSSOHandler) RegisterAuthed(authed chi.Router) {
	authed.Post("/api/v1/admin/sso-providers", h.handleCreate)
	authed.Get("/api/v1/admin/sso-providers", h.handleList)
	authed.Get("/api/v1/admin/sso-providers/{slug}", h.handleGet)
	authed.Patch("/api/v1/admin/sso-providers/{slug}", h.handleUpdate)
	authed.Delete("/api/v1/admin/sso-providers/{slug}", h.handleDelete)
	authed.Post("/api/v1/admin/sso-providers/{slug}/test", h.handleTest)
}

func (h *AdminSSOHandler) RegisterPublic(r chi.Router) {
	r.Get("/api/v1/auth/sso/providers", h.handlePublicList)
}

func (h *AdminSSOHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
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

func (h *AdminSSOHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var body models.SSOProvider
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if !slugRe.MatchString(body.Slug) {
		writeError(w, http.StatusBadRequest, "validation_failed", "slug must match ^[a-z0-9_-]{1,64}$")
		return
	}

	if body.Kind != "oidc" && body.Kind != "saml" {
		writeError(w, http.StatusBadRequest, "validation_failed", "kind must be 'oidc' or 'saml'")
		return
	}

	if body.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "display_name is required")
		return
	}

	if body.Kind == "oidc" {
		var cfg models.OIDCConfig
		if err := json.Unmarshal(body.OIDCConfig, &cfg); err != nil || cfg.Issuer == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "oidc_config requires issuer, client_id, and client_secret")
			return
		}
	}

	if body.Kind == "saml" {
		var cfg models.SAMLConfig
		if err := json.Unmarshal(body.SAMLConfig, &cfg); err != nil || (cfg.IDPMetadataURL == "" && cfg.IDPMetadataXML == "") {
			writeError(w, http.StatusBadRequest, "validation_failed", "saml_config requires idp_metadata_url or idp_metadata_xml")
			return
		}
	}

	now := time.Now()
	body.ID = uuid.New().String()
	body.CreatedAt = now
	body.UpdatedAt = now
	body.Enabled = true

	if err := h.db.CreateSSOProvider(r.Context(), &body); err != nil {
		h.log.Errorw("admin sso create: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, ssoProviderToResponse(&body))
}

func (h *AdminSSOHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	providers, err := h.db.ListAllSSOProviders(r.Context())
	if err != nil {
		h.log.Errorw("admin sso list: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	resp := make([]any, 0, len(providers))
	for _, p := range providers {
		resp = append(resp, ssoProviderToResponse(p))
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": resp})
}

func (h *AdminSSOHandler) handlePublicList(w http.ResponseWriter, r *http.Request) {
	providers, err := h.db.ListEnabledSSOProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	resp := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		resp = append(resp, map[string]any{
			"slug":         p.Slug,
			"kind":         p.Kind,
			"display_name": p.DisplayName,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": resp})
}

func (h *AdminSSOHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	slug := urlparams.Vars(r)["slug"]

	provider, err := h.db.GetSSOProviderBySlug(r.Context(), slug)
	if err != nil {
		h.log.Errorw("admin sso get: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if provider == nil {
		writeError(w, http.StatusNotFound, "not_found", "SSO provider not found")
		return
	}

	writeJSON(w, http.StatusOK, ssoProviderToResponse(provider))
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *AdminSSOHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	slug := urlparams.Vars(r)["slug"]
	ctx := r.Context()

	existing, err := h.db.GetSSOProviderBySlug(ctx, slug)
	if err != nil {
		h.log.Errorw("admin sso update: db get", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "SSO provider not found")
		return
	}

	var patch map[string]json.RawMessage
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if v, ok := patch["display_name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			existing.DisplayName = s
		}
	}

	if v, ok := patch["enabled"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			existing.Enabled = b
		}
	}

	if v, ok := patch["oidc_config"]; ok {
		existing.OIDCConfig = v
	}

	if v, ok := patch["saml_config"]; ok {
		existing.SAMLConfig = v
	}

	if v, ok := patch["claim_mapping"]; ok {
		existing.ClaimMapping = v
	}

	if v, ok := patch["group_mapping"]; ok {
		var gm map[string]string
		if err := json.Unmarshal(v, &gm); err == nil {
			existing.GroupMapping = gm
		}
	}

	if v, ok := patch["permission_mapping"]; ok {
		var pm map[string]string
		if err := json.Unmarshal(v, &pm); err == nil {
			existing.PermissionMapping = pm
		}
	}

	if v, ok := patch["auto_provision_users"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			existing.AutoProvisionUsers = b
		}
	}

	if v, ok := patch["link_by_email"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			existing.LinkByEmail = b
		}
	}

	existing.UpdatedAt = time.Now()

	if err := h.db.UpdateSSOProvider(ctx, existing); err != nil {
		h.log.Errorw("admin sso update: db update", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, ssoProviderToResponse(existing))
}

func (h *AdminSSOHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	slug := urlparams.Vars(r)["slug"]
	ctx := r.Context()

	existing, err := h.db.GetSSOProviderBySlug(ctx, slug)
	if err != nil {
		h.log.Errorw("admin sso delete: db get", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "SSO provider not found")
		return
	}

	h.log.Warnw("admin sso: hard deleting SSO provider", "slug", slug, "id", existing.ID)

	if err := h.db.DeleteSSOProvider(ctx, existing.ID); err != nil {
		h.log.Errorw("admin sso delete: db delete", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *AdminSSOHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	slug := urlparams.Vars(r)["slug"]
	ctx := r.Context()

	existing, err := h.db.GetSSOProviderBySlug(ctx, slug)
	if err != nil {
		h.log.Errorw("admin sso test: db get", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "SSO provider not found")
		return
	}

	switch existing.Kind {
	case "oidc":
		_, err := auth.LoadOIDCProvider(ctx, h.db, slug, h.baseURL)
		if err != nil {
			if errors.Is(err, auth.ErrOIDCDiscoveryFailed) {
				writeJSON(w, http.StatusOK, map[string]any{
					"ok":      false,
					"error":   "oidc_discovery_failed",
					"message": err.Error(),
				})

				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      false,
				"error":   "load_failed",
				"message": err.Error(),
			})

			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kind": "oidc"})

	case "saml":
		_, err := auth.LoadSAMLProvider(ctx, h.db, slug, h.baseURL, h.cfg, h.log)
		if err != nil {
			if errors.Is(err, auth.ErrIDPMetadataUnavailable) {
				writeJSON(w, http.StatusOK, map[string]any{
					"ok":      false,
					"error":   "idp_metadata_unavailable",
					"message": err.Error(),
				})

				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      false,
				"error":   "load_failed",
				"message": err.Error(),
			})

			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kind": "saml"})

	default:
		writeError(w, http.StatusBadRequest, "unknown_kind", "unknown provider kind")
	}
}

func ssoProviderToResponse(p *models.SSOProvider) responses.SSOProviderResponse {
	var oidcCfg map[string]any

	if len(p.OIDCConfig) > 0 {
		var cfg models.OIDCConfig
		if err := json.Unmarshal(p.OIDCConfig, &cfg); err == nil {
			cfg.ClientSecret = ""
			if b, err := json.Marshal(cfg); err == nil { //nolint:gosec // G117: client_secret is intentionally marshaled for storage
				_ = json.Unmarshal(b, &oidcCfg)
			}
		}
	}

	var samlCfg map[string]any
	if len(p.SAMLConfig) > 0 {
		_ = json.Unmarshal(p.SAMLConfig, &samlCfg)
	}

	var claimMapping map[string]any
	if len(p.ClaimMapping) > 0 {
		_ = json.Unmarshal(p.ClaimMapping, &claimMapping)
	}

	return responses.SSOProviderResponse{
		ID:                 p.ID,
		Slug:               p.Slug,
		Kind:               p.Kind,
		DisplayName:        p.DisplayName,
		Enabled:            p.Enabled,
		OIDCConfig:         oidcCfg,
		SAMLConfig:         samlCfg,
		ClaimMapping:       claimMapping,
		GroupMapping:       p.GroupMapping,
		PermissionMapping:  p.PermissionMapping,
		AutoProvisionUsers: p.AutoProvisionUsers,
		LinkByEmail:        p.LinkByEmail,
		CreatedAt:          formatTime(p.CreatedAt),
		UpdatedAt:          formatTime(p.UpdatedAt),
	}
}
