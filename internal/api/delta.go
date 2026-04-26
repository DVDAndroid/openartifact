// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

// DeltaHandler provides HTTP endpoints for binary diff (delta) distribution.
//
// Endpoints:
//   GET  /api/v1/repositories/{name}/packages/{id}/delta?from={sha256}
//        Returns a pre-computed bsdiff patch that transforms the package
//        identified by "from" SHA-256 into the package identified by "id".
//        If no pre-computed delta exists, one is generated on-the-fly,
//        stored for future requests, and streamed to the client.
//
//   GET  /api/v1/repositories/{name}/packages/{id}/deltas
//        Lists all available deltas targeting the package identified by "id".

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	database "github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	pkgdelta "github.com/openartifact/openartifact/pkg/delta"
	"github.com/openartifact/openartifact/pkg/logger"
)

// DeltaHandler handles package delta requests.
type DeltaHandler struct {
	log       *logger.Logger
	db        database.Database
	blobStore storage.BlobStore
}

// NewDeltaHandler creates a new DeltaHandler.
func NewDeltaHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore) *DeltaHandler {
	return &DeltaHandler{log: log, db: db, blobStore: blobStore}
}

// Register registers the delta routes on the given router.
func (h *DeltaHandler) Register(router chi.Router) {
	router.Get("/api/v1/repositories/{name}/packages/{id}/delta", h.handleGetDelta)
	router.Get("/api/v1/repositories/{name}/packages/{id}/deltas", h.handleListDeltas)
	h.log.Infow("Delta handler registered")
}

// handleGetDelta serves a binary diff patch.
// Query parameter: from=<sha256> — the SHA-256 of the package the client already has.
func (h *DeltaHandler) handleGetDelta(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	toID := vars["id"] // component ID of the target package

	fromSHA256 := r.URL.Query().Get("from")
	if fromSHA256 == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "missing 'from' query parameter (sha256 of source package)")
		return
	}

	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	// Look up the target component.
	toComp, err := h.db.GetComponentByID(r.Context(), toID)
	if err != nil || toComp == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "target package not found")
		return
	}

	// Check for a pre-computed delta.
	existing, err := h.db.GetPackageDelta(r.Context(), repo.ID, fromSHA256, toComp.SHA256)
	if err != nil {
		h.log.Errorw("failed to query delta", "error", err)
		writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to query delta")

		return
	}

	if existing != nil {
		// Serve the pre-computed delta blob.
		h.serveDeltaBlob(w, r, existing)
		return
	}

	// No pre-computed delta — generate on-the-fly.
	delta, err := h.generateDelta(r, repo.ID, fromSHA256, toComp)
	if err != nil {
		h.log.Errorw("failed to generate delta", "from", fromSHA256, "to", toComp.SHA256, "error", err)
		writeError(w, http.StatusInternalServerError, "DELTA_ERROR", "failed to generate delta")

		return
	}

	h.serveDeltaBlob(w, r, delta)
}

// handleListDeltas lists all available deltas targeting a package.
func (h *DeltaHandler) handleListDeltas(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	toID := vars["id"]

	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	toComp, err := h.db.GetComponentByID(r.Context(), toID)
	if err != nil || toComp == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "target package not found")
		return
	}

	deltas, err := h.db.ListPackageDeltas(r.Context(), repo.ID, toComp.SHA256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to list deltas")
		return
	}

	type deltaInfo struct {
		FromSHA256   string `json:"from_sha256"`
		FromFilename string `json:"from_filename"`
		ToSHA256     string `json:"to_sha256"`
		ToFilename   string `json:"to_filename"`
		DeltaSize    int64  `json:"delta_size"`
		Format       string `json:"format"`
	}

	result := make([]deltaInfo, 0, len(deltas))
	for _, d := range deltas {
		result = append(result, deltaInfo{
			FromSHA256:   d.FromSHA256,
			FromFilename: d.FromFilename,
			ToSHA256:     d.ToSHA256,
			ToFilename:   d.ToFilename,
			DeltaSize:    d.DeltaSize,
			Format:       d.Format,
		})
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: result})
}

// generateDelta generates a bsdiff patch from fromSHA256 to toComp, stores it,
// and returns the PackageDelta record.
func (h *DeltaHandler) generateDelta(r *http.Request, repoID, fromSHA256 string, toComp *models.Component) (*models.PackageDelta, error) {
	// Fetch the "from" blob.
	fromReader, err := h.blobStore.Get(r.Context(), fromSHA256)
	if err != nil {
		return nil, fmt.Errorf("fetch from-blob %s: %w", fromSHA256, err)
	}
	defer func() { base.CloseLog(fromReader, h.log) }()

	fromData, err := io.ReadAll(fromReader)
	if err != nil {
		return nil, fmt.Errorf("read from-blob: %w", err)
	}

	// Fetch the "to" blob.
	toReader, err := h.blobStore.Get(r.Context(), toComp.SHA256)
	if err != nil {
		return nil, fmt.Errorf("fetch to-blob %s: %w", toComp.SHA256, err)
	}
	defer func() { base.CloseLog(toReader, h.log) }()

	toData, err := io.ReadAll(toReader)
	if err != nil {
		return nil, fmt.Errorf("read to-blob: %w", err)
	}

	// Generate the binary diff.
	patch, patchSHA256, err := pkgdelta.Generate(fromData, toData)
	if err != nil {
		return nil, fmt.Errorf("generate diff: %w", err)
	}

	// Store the delta blob.
	_, err = h.blobStore.Put(r.Context(), bytes.NewReader(patch), map[string]string{
		"sha256": patchSHA256,
		"type":   "delta",
	})
	if err != nil {
		return nil, fmt.Errorf("store delta blob: %w", err)
	}

	// Look up the from-component to get its filename.
	fromComp, err := h.db.GetComponentsByRepo(r.Context(), repoID)
	fromFilename := fromSHA256 // fallback

	if err == nil {
		for _, c := range fromComp {
			if c.SHA256 == fromSHA256 {
				fromFilename = c.Filename
				break
			}
		}
	}

	// Persist the delta record.
	d := &models.PackageDelta{
		RepositoryID: repoID,
		FromSHA256:   fromSHA256,
		FromFilename: fromFilename,
		ToSHA256:     toComp.SHA256,
		ToFilename:   toComp.Filename,
		DeltaSHA256:  patchSHA256,
		DeltaSize:    int64(len(patch)),
		Format:       toComp.Format,
	}
	if err := h.db.CreatePackageDelta(r.Context(), d); err != nil {
		h.log.Warnw("failed to persist delta record", "error", err)
		// Non-fatal: we can still serve the delta even if DB write fails.
	}

	return d, nil
}

// serveDeltaBlob streams a delta blob to the client.
func (h *DeltaHandler) serveDeltaBlob(w http.ResponseWriter, r *http.Request, d *models.PackageDelta) {
	reader, err := h.blobStore.Get(r.Context(), d.DeltaSHA256)
	if err != nil {
		h.log.Errorw("failed to read delta blob", "sha256", d.DeltaSHA256, "error", err)
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to read delta")

		return
	}
	defer func() { base.CloseLog(reader, h.log) }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.delta"`, d.ToFilename))
	w.Header().Set("ETag", `"`+d.DeltaSHA256+`"`)
	w.Header().Set("X-Delta-From", d.FromSHA256)
	w.Header().Set("X-Delta-To", d.ToSHA256)
	w.Header().Set("X-Delta-Size", strconv.FormatInt(d.DeltaSize, 10))
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := io.Copy(w, reader); err != nil {
		h.log.Errorw("failed to write delta response", "error", err)
	}
}
