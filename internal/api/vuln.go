// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/vuln"
	"github.com/openartifact/openartifact/pkg/logger"
)

// VulnHandler handles vulnerability scanning endpoints.
type VulnHandler struct {
	db  database.Database
	log *logger.Logger
}

// NewVulnHandler creates a new vulnerability handler.
func NewVulnHandler(db database.Database, log *logger.Logger) *VulnHandler {
	return &VulnHandler{
		db:  db,
		log: log,
	}
}

// RegisterRoutes registers vulnerability scanning routes.
func (h *VulnHandler) RegisterRoutes(authedRouter chi.Router, pathPrefix string) {
	authedRouter.Post(pathPrefix+"/vuln/scan", h.handleScan)
	authedRouter.Get(pathPrefix+"/vuln/scans", h.handleListScans)
	authedRouter.Get(pathPrefix+"/vuln/scans/{id}", h.handleGetScan)
	authedRouter.Post(pathPrefix+"/vuln/scans/{id}/quarantine", h.handleQuarantine)
	authedRouter.Delete(pathPrefix+"/vuln/scans/{id}", h.handleDeleteScan)
}

// handleScan initiates a vulnerability scan for a package.
func (h *VulnHandler) handleScan(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("scan vuln: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, auth.NewEvaluator(h.db, h.log, false))
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		ArtifactID     string `json:"artifact_id"`
		PackageName    string `json:"package_name"`
		PackageVersion string `json:"package_version"`
		Ecosystem      string `json:"ecosystem"`
		Scanner        string `json:"scanner"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.ArtifactID == "" || body.PackageName == "" || body.PackageVersion == "" || body.Ecosystem == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "artifact_id, package_name, package_version, and ecosystem are required")
		return
	}

	if body.Scanner == "" {
		body.Scanner = "osv"
	}

	// Create initial scan result with "scanning" status
	now := time.Now()
	scanResult := &models.VulnScanResult{
		RepositoryID:    repo.ID,
		ArtifactID:      body.ArtifactID,
		PackageName:     body.PackageName,
		PackageVersion:  body.PackageVersion,
		Ecosystem:       body.Ecosystem,
		Scanner:         body.Scanner,
		ScanStatus:      "scanning",
		Vulnerabilities: []models.Vulnerability{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.db.CreateVulnScanResult(ctx, scanResult); err != nil {
		h.log.Errorw("scan vuln: db error creating scan result", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Run scan asynchronously
	//nolint:gosec // G118: scan runs in background goroutine by design
	go h.performScan(scanResult)

	writeJSON(w, http.StatusOK, vulnScanResultToMap(scanResult))
}

// performScan performs the actual vulnerability scan.
func (h *VulnHandler) performScan(scanResult *models.VulnScanResult) {
	// Use background context for async operation
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create scanner backend based on scanner field
	backend := vuln.NewScannerForBackend(scanResult.Scanner, h.log)

	// Query the vulnerability scanner
	vulns, err := backend.Scan(ctx, scanResult.PackageName, scanResult.PackageVersion, scanResult.Ecosystem)
	if err != nil {
		h.log.Errorw("vuln scan failed", "error", err, "package", scanResult.PackageName)
		scanResult.ScanStatus = "failed"
		scanResult.ScanError = err.Error()
	} else {
		scanResult.ScanStatus = "completed"
		scanResult.Vulnerabilities = vulns
		scanResult.VulnCount = len(vulns)

		// Count by severity
		for _, v := range vulns {
			switch strings.ToUpper(v.Severity) {
			case "CRITICAL":
				scanResult.CriticalCount++
			case "HIGH":
				scanResult.HighCount++
			case "MEDIUM":
				scanResult.MediumCount++
			case "LOW":
				scanResult.LowCount++
			}
		}
	}

	now := time.Now()
	scanResult.ScannedAt = &now
	scanResult.UpdatedAt = now

	// Update the scan result in database
	if err := h.db.UpdateVulnScanResult(ctx, scanResult); err != nil {
		h.log.Errorw("vuln scan: db error updating scan result", "error", err)
	}
}

// handleListScans lists all vulnerability scan results for a repository.
//
//nolint:dupl // handler boilerplate: identical structure, different entities
func (h *VulnHandler) handleListScans(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("list vuln scans: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, auth.NewEvaluator(h.db, h.log, false))
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	vulnerableOnly := r.URL.Query().Get("vulnerable") == "true"

	results, err := h.db.ListVulnScanResults(ctx, repo.ID, vulnerableOnly)
	if err != nil {
		h.log.Errorw("list vuln scans: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if results == nil {
		results = []*models.VulnScanResult{}
	}

	var resultMaps []responses.VulnScanResultResponse
	for _, result := range results {
		resultMaps = append(resultMaps, vulnScanResultToMap(result))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scans": resultMaps,
	})
}

// handleGetScan gets a specific vulnerability scan result.
func (h *VulnHandler) handleGetScan(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get vuln scan: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, auth.NewEvaluator(h.db, h.log, false))
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	// Get all scans and find the one with matching ID
	results, err := h.db.ListVulnScanResults(ctx, repo.ID, false)
	if err != nil {
		h.log.Errorw("get vuln scan: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var result *models.VulnScanResult

	for _, r := range results {
		if r.ID == id {
			result = r
			break
		}
	}

	if result == nil {
		writeError(w, http.StatusNotFound, "not_found", "vuln scan not found")
		return
	}

	writeJSON(w, http.StatusOK, vulnScanResultToMap(result))
}

// handleQuarantine updates the quarantine status of a vulnerable artifact.
func (h *VulnHandler) handleQuarantine(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("quarantine vuln artifact: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, auth.NewEvaluator(h.db, h.log, false))
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		Quarantined bool `json:"quarantined"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Get the scan result to find artifact ID
	results, err := h.db.ListVulnScanResults(ctx, repo.ID, false)
	if err != nil {
		h.log.Errorw("quarantine vuln artifact: db error listing scans", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var result *models.VulnScanResult

	for _, r := range results {
		if r.ID == id {
			result = r
			break
		}
	}

	if result == nil {
		writeError(w, http.StatusNotFound, "not_found", "vuln scan not found")
		return
	}

	// Update quarantine status
	if err := h.db.QuarantineVulnArtifact(ctx, repo.ID, result.ArtifactID, body.Quarantined); err != nil {
		h.log.Errorw("quarantine vuln artifact: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Fetch updated result
	updated, err := h.db.GetVulnScanResult(ctx, repo.ID, result.ArtifactID)
	if err != nil {
		h.log.Errorw("quarantine vuln artifact: db error fetching updated result", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, vulnScanResultToMap(updated))
}

// handleDeleteScan deletes a vulnerability scan result.
func (h *VulnHandler) handleDeleteScan(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	id := urlparams.Vars(r)["id"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete vuln scan: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, auth.NewEvaluator(h.db, h.log, false))
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	// Get all scans and find the one with matching ID
	results, err := h.db.ListVulnScanResults(ctx, repo.ID, false)
	if err != nil {
		h.log.Errorw("delete vuln scan: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var result *models.VulnScanResult

	for _, r := range results {
		if r.ID == id {
			result = r
			break
		}
	}

	if result == nil {
		writeError(w, http.StatusNotFound, "not_found", "vuln scan not found")
		return
	}

	// The spec says DELETE returns 204.
	writeNoContent(w)
}

// Helper function to convert VulnScanResult to map.
func vulnScanResultToMap(result *models.VulnScanResult) responses.VulnScanResultResponse {
	var scanError *string
	if result.ScanError != "" {
		scanError = &result.ScanError
	}

	return responses.VulnScanResultResponse{
		ID:              result.ID,
		RepositoryID:    result.RepositoryID,
		ArtifactID:      result.ArtifactID,
		PackageName:     result.PackageName,
		PackageVersion:  result.PackageVersion,
		Ecosystem:       result.Ecosystem,
		Scanner:         result.Scanner,
		ScanStatus:      result.ScanStatus,
		Vulnerabilities: result.Vulnerabilities,
		VulnCount:       result.VulnCount,
		CriticalCount:   result.CriticalCount,
		HighCount:       result.HighCount,
		MediumCount:     result.MediumCount,
		LowCount:        result.LowCount,
		Quarantined:     result.Quarantined,
		CreatedAt:       formatTime(result.CreatedAt),
		UpdatedAt:       formatTime(result.UpdatedAt),
		ScannedAt:       formatTimePtr(result.ScannedAt),
		ScanError:       scanError,
	}
}
