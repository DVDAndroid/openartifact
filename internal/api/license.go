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
	"github.com/openartifact/openartifact/internal/license"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type LicenseHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
	detector  *license.Detector
}

func NewLicenseHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator, detector *license.Detector) *LicenseHandler {
	return &LicenseHandler{
		db:        db,
		log:       log,
		evaluator: evaluator,
		detector:  detector,
	}
}

func (h *LicenseHandler) RegisterRoutes(authedRouter chi.Router) {
	// License scan results
	authedRouter.Post("/api/v1/repositories/{name}/license/scan", h.handleScan)
	authedRouter.Get("/api/v1/repositories/{name}/license/scans", h.handleListScans)
	authedRouter.Get("/api/v1/repositories/{name}/license/scans/{id}", h.handleGetScan)

	// License policies
	authedRouter.Get("/api/v1/repositories/{name}/license/policies", h.handleListPolicies)
	authedRouter.Post("/api/v1/repositories/{name}/license/policies", h.handleCreatePolicy)
	authedRouter.Put("/api/v1/repositories/{name}/license/policies/{id}", h.handleUpdatePolicy)
	authedRouter.Delete("/api/v1/repositories/{name}/license/policies/{id}", h.handleDeletePolicy)

	// Global policies (admin)
	authedRouter.Get("/api/v1/admin/license/policies", h.handleListGlobalPolicies)
	authedRouter.Post("/api/v1/admin/license/policies", h.handleCreateGlobalPolicy)
}

// handleScan scans an artifact for licenses and evaluates policies.
func (h *LicenseHandler) handleScan(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("scan licenses: db error getting repo", "error", err)
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
		ArtifactID string         `json:"artifact_id"`
		Metadata   map[string]any `json:"metadata"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.ArtifactID == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "artifact_id is required")
		return
	}

	// Detect licenses from metadata
	licenses := h.detector.DetectFromMetadata(body.Metadata)

	// Create scan result
	now := time.Now()
	scanResult := &models.LicenseScanResult{
		RepositoryID: repo.ID,
		ArtifactID:   body.ArtifactID,
		Licenses:     licenses,
		ScanStatus:   "completed",
		ScannedAt:    &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.db.CreateLicenseScanResult(ctx, scanResult); err != nil {
		h.log.Errorw("scan licenses: db error creating scan result", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Evaluate policies
	policies, err := h.db.ListLicensePolicies(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("scan licenses: db error listing policies", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var policyViolations []map[string]any

	for _, policy := range policies {
		compliant, violations := h.detector.EvaluatePolicy(licenses, policy)
		if !compliant {
			for _, violation := range violations {
				policyViolations = append(policyViolations, map[string]any{
					"policy_id":   policy.ID,
					"policy_name": policy.Name,
					"action":      policy.Action,
					"violation":   violation,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                scanResult.ID,
		"repo_id":           scanResult.RepositoryID,
		"artifact_id":       scanResult.ArtifactID,
		"licenses":          scanResult.Licenses,
		"scan_status":       scanResult.ScanStatus,
		"scanned_at":        scanResult.ScannedAt,
		"created_at":        scanResult.CreatedAt,
		"updated_at":        scanResult.UpdatedAt,
		"policy_violations": policyViolations,
	})
}

// handleListScans lists all license scan results for a repository.
//
//nolint:dupl // handler boilerplate: identical structure, different entities
func (h *LicenseHandler) handleListScans(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("list license scans: db error getting repo", "error", err)
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

	results, err := h.db.ListLicenseScanResults(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("list license scans: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if results == nil {
		results = []*models.LicenseScanResult{}
	}

	var resultMaps []responses.LicenseScanResultResponse
	for _, result := range results {
		resultMaps = append(resultMaps, scanResultToMap(result))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scans": resultMaps,
	})
}

// handleGetScan gets a specific license scan result.
func (h *LicenseHandler) handleGetScan(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get license scan: db error getting repo", "error", err)
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

	// Get all scans and find the one with matching ID
	results, err := h.db.ListLicenseScanResults(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("get license scan: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var result *models.LicenseScanResult

	for _, r := range results {
		if r.ID == id {
			result = r
			break
		}
	}

	if result == nil {
		writeError(w, http.StatusNotFound, "not_found", "license scan not found")
		return
	}

	writeJSON(w, http.StatusOK, scanResultToMap(result))
}

// handleListPolicies lists all license policies for a repository.
//
//nolint:dupl // handler boilerplate: identical structure, different entities
func (h *LicenseHandler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("list license policies: db error getting repo", "error", err)
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

	policies, err := h.db.ListLicensePolicies(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("list license policies: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policies == nil {
		policies = []*models.LicensePolicy{}
	}

	var policyMaps []responses.LicensePolicyResponse
	for _, policy := range policies {
		policyMaps = append(policyMaps, licensePolicyToMap(policy))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policies": policyMaps,
	})
}

// handleCreatePolicy creates a new license policy.
func (h *LicenseHandler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("create license policy: db error getting repo", "error", err)
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
		Name            string   `json:"name"`
		Description     string   `json:"description"`
		Action          string   `json:"action"`
		AllowedLicenses []string `json:"allowed_licenses"`
		DeniedLicenses  []string `json:"denied_licenses"`
		Enabled         bool     `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}

	if body.Action == "" {
		body.Action = "warn"
	}

	policy := &models.LicensePolicy{
		RepositoryID:    &repo.ID,
		Name:            body.Name,
		Description:     body.Description,
		Action:          body.Action,
		AllowedLicenses: body.AllowedLicenses,
		DeniedLicenses:  body.DeniedLicenses,
		Enabled:         body.Enabled,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.db.CreateLicensePolicy(ctx, policy); err != nil {
		h.log.Errorw("create license policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, licensePolicyToMap(policy))
}

// handleUpdatePolicy updates a license policy.
func (h *LicenseHandler) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("update license policy: db error getting repo", "error", err)
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

	policy, err := h.db.GetLicensePolicy(ctx, id)
	if err != nil {
		h.log.Errorw("update license policy: db error getting policy", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policy == nil || (policy.RepositoryID != nil && *policy.RepositoryID != repo.ID) {
		writeError(w, http.StatusNotFound, "not_found", "license policy not found")
		return
	}

	var body struct {
		Name            *string  `json:"name"`
		Description     *string  `json:"description"`
		Action          *string  `json:"action"`
		AllowedLicenses []string `json:"allowed_licenses"`
		DeniedLicenses  []string `json:"denied_licenses"`
		Enabled         *bool    `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name != nil {
		policy.Name = *body.Name
	}

	if body.Description != nil {
		policy.Description = *body.Description
	}

	if body.Action != nil {
		policy.Action = *body.Action
	}

	if body.AllowedLicenses != nil {
		policy.AllowedLicenses = body.AllowedLicenses
	}

	if body.DeniedLicenses != nil {
		policy.DeniedLicenses = body.DeniedLicenses
	}

	if body.Enabled != nil {
		policy.Enabled = *body.Enabled
	}

	policy.UpdatedAt = time.Now()

	if err := h.db.UpdateLicensePolicy(ctx, policy); err != nil {
		h.log.Errorw("update license policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, licensePolicyToMap(policy))
}

// handleDeletePolicy deletes a license policy.
func (h *LicenseHandler) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete license policy: db error getting repo", "error", err)
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

	policy, err := h.db.GetLicensePolicy(ctx, id)
	if err != nil {
		h.log.Errorw("delete license policy: db error getting policy", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if policy == nil || (policy.RepositoryID != nil && *policy.RepositoryID != repo.ID) {
		writeError(w, http.StatusNotFound, "not_found", "license policy not found")
		return
	}

	if err := h.db.DeleteLicensePolicy(ctx, id); err != nil {
		h.log.Errorw("delete license policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

// handleListGlobalPolicies lists all global license policies.
func (h *LicenseHandler) handleListGlobalPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	// Query for global policies (repo_id IS NULL)
	query := `
		SELECT id, repo_id, name, description, action, allowed_licenses, denied_licenses, enabled, created_at, updated_at
		FROM license_policies
		WHERE repo_id IS NULL
		ORDER BY created_at DESC
	`

	rows, err := h.db.GetDB().QueryContext(ctx, query)
	if err != nil {
		h.log.Errorw("list global license policies: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}
	defer func() { _ = rows.Close() }()

	var policies []*models.LicensePolicy

	for rows.Next() {
		policy := &models.LicensePolicy{}

		var repoID *string

		err := rows.Scan(
			&policy.ID,
			&repoID,
			&policy.Name,
			&policy.Description,
			&policy.Action,
			// arrays handled by pgx stdlib natively
		)
		if err != nil {
			h.log.Errorw("list global license policies: scan error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}

		policy.RepositoryID = repoID
		policies = append(policies, policy)
	}

	if policies == nil {
		policies = []*models.LicensePolicy{}
	}

	var policyMaps []responses.LicensePolicyResponse
	for _, policy := range policies {
		policyMaps = append(policyMaps, licensePolicyToMap(policy))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policies": policyMaps,
	})
}

// handleCreateGlobalPolicy creates a new global license policy.
func (h *LicenseHandler) handleCreateGlobalPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		Name            string   `json:"name"`
		Description     string   `json:"description"`
		Action          string   `json:"action"`
		AllowedLicenses []string `json:"allowed_licenses"`
		DeniedLicenses  []string `json:"denied_licenses"`
		Enabled         bool     `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}

	if body.Action == "" {
		body.Action = "warn"
	}

	policy := &models.LicensePolicy{
		RepositoryID:    nil, // Global policy
		Name:            body.Name,
		Description:     body.Description,
		Action:          body.Action,
		AllowedLicenses: body.AllowedLicenses,
		DeniedLicenses:  body.DeniedLicenses,
		Enabled:         body.Enabled,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.db.CreateLicensePolicy(ctx, policy); err != nil {
		h.log.Errorw("create global license policy: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, licensePolicyToMap(policy))
}

// Helper functions

func scanResultToMap(result *models.LicenseScanResult) responses.LicenseScanResultResponse {
	var scanError *string
	if result.ScanError != "" {
		scanError = &result.ScanError
	}

	return responses.LicenseScanResultResponse{
		ID:              result.ID,
		RepositoryID:    result.RepositoryID,
		ArtifactID:      result.ArtifactID,
		Licenses:        result.Licenses,
		ScanStatus:      result.ScanStatus,
		CreatedAt:       formatTime(result.CreatedAt),
		UpdatedAt:       formatTime(result.UpdatedAt),
		ScannedAt:       formatTimePtr(result.ScannedAt),
		ScanError:       scanError,
		SPDXExpressions: result.SPDXExpressions,
	}
}

func licensePolicyToMap(policy *models.LicensePolicy) responses.LicensePolicyResponse {
	description := policy.Description
	action := policy.Action

	return responses.LicensePolicyResponse{
		ID:              policy.ID,
		Name:            policy.Name,
		Description:     &description,
		Action:          &action,
		AllowedLicenses: policy.AllowedLicenses,
		DeniedLicenses:  policy.DeniedLicenses,
		Enabled:         policy.Enabled,
		CreatedAt:       formatTime(policy.CreatedAt),
		UpdatedAt:       formatTime(policy.UpdatedAt),
		RepositoryID:    policy.RepositoryID,
	}
}
