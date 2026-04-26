// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	database "github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/worker"
	pkgdeb "github.com/openartifact/openartifact/pkg/deb"
	"github.com/openartifact/openartifact/pkg/logger"
)

const maxDebUploadSize = 500 * 1024 * 1024

type DebUploadHandler struct {
	log       *logger.Logger
	db        database.Database
	blobStore storage.BlobStore
	reg       *metrics.Registry
	debWorker *worker.DebMetadataWorker
}

func NewDebUploadHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, reg *metrics.Registry) *DebUploadHandler {
	return &DebUploadHandler{
		log:       log,
		db:        db,
		blobStore: blobStore,
		reg:       reg,
	}
}

func (h *DebUploadHandler) SetDebWorker(w *worker.DebMetadataWorker) {
	h.debWorker = w
}

func (h *DebUploadHandler) Register(router chi.Router) {
	router.Post("/api/v1/repositories/{name}/debs", h.handleUpload)
	router.Delete("/api/v1/repositories/{name}/debs/{id}", h.handleDelete)
	h.log.Infow("Deb upload handler registered")
}

func (h *DebUploadHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

	uploadResult := "success"

	if h.reg != nil {
		defer func() { h.reg.UploadsTotal.WithLabelValues("deb", uploadResult).Inc() }()
	}

	if err := r.ParseMultipartForm(maxDebUploadSize); err != nil { //nolint:gosec // G120: form size is bounded by maxDebUploadSize
		uploadResult = "error"

		writeError(w, http.StatusBadRequest, "INVALID_INPUT", fmt.Sprintf("failed to parse form: %v", err))

		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		uploadResult = "error"

		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "missing file field")

		return
	}
	defer func() { base.CloseLog(file, h.log) }()

	if header.Size > maxDebUploadSize {
		uploadResult = "error"

		writeError(w, http.StatusRequestEntityTooLarge, "INVALID_INPUT", "file exceeds maximum size limit")

		return
	}

	distribution := r.FormValue("distribution")
	if distribution == "" {
		distribution = "stable"
	}

	component := r.FormValue("component")
	if component == "" {
		component = "main"
	}

	result, err := h.uploadPipeline(r, file, repoName, distribution, component)
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("Deb upload failed", "repository", repoName, "filename", header.Filename, "error", err)
		writeError(w, http.StatusInternalServerError, "UPLOAD_ERROR", "upload processing failed")

		return
	}

	if dup, _ := result["duplicate"].(bool); dup {
		uploadResult = "error"

		writeJSON(w, http.StatusConflict, Response{
			Success: false,
			Error:   "artifact already exists",
			Code:    "CONFLICT",
			Data: map[string]any{
				"duplicate": true,
			},
		})

		return
	}

	h.log.Infow("Deb package uploaded", "repository", repoName, "package", result["name"], "version", result["version"])
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    result,
	})
}

func (h *DebUploadHandler) resolveRepoID(r *http.Request, repoName string) (string, error) {
	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil {
		return "", fmt.Errorf("failed to look up repository: %w", err)
	}

	if repo == nil {
		return "", fmt.Errorf("repository not found: %s", repoName) //nolint:err113 // dynamic error with runtime context
	}

	return repo.ID, nil
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *DebUploadHandler) uploadPipeline(r *http.Request, file io.Reader, repoName string, distribution string, component string) (map[string]any, error) {
	repoID, err := h.resolveRepoID(r, repoName)
	if err != nil {
		return nil, err
	}

	// Stream to temp file while computing SHA256
	tmpFile, err := os.CreateTemp("", "deb-upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { base.CloseLog(tmpFile, h.log) }()

	hasher := sha256.New()

	written, err := io.Copy(tmpFile, io.TeeReader(file, hasher))
	if err != nil {
		return nil, fmt.Errorf("failed to buffer upload: %w", err)
	}

	sha256sum := hex.EncodeToString(hasher.Sum(nil))

	// Per-repo duplicate check: same package already in this repo.
	exists, err := h.db.ArtifactExists(r.Context(), sha256sum, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to check artifact existence: %w", err)
	}

	if exists {
		return map[string]any{"duplicate": true}, nil
	}

	// Parse from temp file
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	debFile, err := pkgdeb.ParseDebFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deb: %w", err)
	}

	pkg := debFile.Package
	filename := fmt.Sprintf("%s_%s_%s.deb", pkg.Package, pkg.Version, pkg.Architecture)

	// Cross-repo dedup: if the blob already exists in any repo, skip re-uploading it.
	// The component record is still created for this repo, pointing to the shared blob.
	blobID, blobExists, err := h.blobStore.ExistsBySHA256(r.Context(), sha256sum)
	if err != nil {
		return nil, fmt.Errorf("failed to check blob existence: %w", err)
	}

	if !blobExists {
		// Store blob from temp file
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek temp file: %w", err)
		}

		blobID, err = h.blobStore.Put(r.Context(), tmpFile, map[string]string{
			"sha256":    sha256sum,
			"format":    "deb",
			"repo_name": repoName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to store blob: %w", err)
		}
	}

	// DB operations — clean up blob on failure only if we uploaded it (not a cross-repo shared blob).
	cleanupBlob := func() {
		if blobExists {
			return // blob was already present; don't delete a shared blob
		}

		if delErr := h.blobStore.Delete(r.Context(), blobID); delErr != nil {
			h.log.Warnw("failed to clean up blob after DB error", "blob_id", blobID, "error", delErr)
		}
	}

	dist, err := h.db.GetDebianDistribution(r.Context(), repoID, distribution)
	if err != nil {
		cleanupBlob()
		return nil, fmt.Errorf("failed to get distribution: %w", err)
	}

	if dist == nil {
		dist = &models.DebianDistribution{
			RepositoryID: repoID,
			Name:         distribution,
		}
		if err := h.db.CreateDebianDistribution(r.Context(), dist); err != nil {
			cleanupBlob()
			return nil, fmt.Errorf("failed to create distribution: %w", err)
		}
	}

	comps, err := h.db.GetDebianComponentsByDist(r.Context(), dist.ID)
	if err != nil {
		cleanupBlob()
		return nil, fmt.Errorf("failed to get components: %w", err)
	}

	var componentID string
	found := false

	for _, c := range comps {
		if c.Name == component {
			found = true
			componentID = c.ID
			break
		}
	}

	if !found {
		debComp := &models.DebianComponent{
			DistributionID: dist.ID,
			Name:           component,
		}
		if err := h.db.CreateDebianComponent(r.Context(), debComp); err != nil {
			cleanupBlob()
			return nil, fmt.Errorf("failed to create component: %w", err)
		}
		componentID = debComp.ID
	}

	// Ensure architecture entry exists for this distribution
	archs, err := h.db.GetDebianArchitecturesByDist(r.Context(), dist.ID)
	if err != nil {
		cleanupBlob()
		return nil, fmt.Errorf("failed to get architectures: %w", err)
	}

	archFound := false

	for _, a := range archs {
		if a.Name == pkg.Architecture {
			archFound = true
			break
		}
	}

	if !archFound {
		debArch := &models.DebianArchitecture{
			DistributionID: dist.ID,
			Name:           pkg.Architecture,
		}
		if err := h.db.CreateDebianArchitecture(r.Context(), debArch); err != nil {
			cleanupBlob()
			return nil, fmt.Errorf("failed to create architecture: %w", err)
		}
	}

	description := pkg.Description
	if idx := strings.Index(description, "\n"); idx != -1 {
		description = description[:idx]
	}

	// Upsert into blobs table to get the UUID for blob_id FK reference.
	blobUUID, err := h.db.UpsertBlob(r.Context(), sha256sum, sha256sum, written)
	if err != nil {
		cleanupBlob()
		return nil, fmt.Errorf("failed to upsert blob record: %w", err)
	}

	comp := &models.Component{
		RepositoryID:  repoID,
		ArtifactID:    blobUUID,
		Name:          pkg.Package,
		Version:       pkg.Version,
		Arch:          pkg.Architecture,
		Summary:       description,
		Description:   pkg.Description,
		InstalledSize: pkg.InstalledSize,
		Filename:      filename,
	}

	if err := h.db.CreateDebPackage(r.Context(), comp, dist.ID, componentID); err != nil {
		cleanupBlob()
		return nil, fmt.Errorf("failed to create component record: %w", err)
	}

	if h.debWorker != nil {
		h.debWorker.NotifyStale(repoID)
	}

	return map[string]any{
		"name":         pkg.Package,
		"version":      pkg.Version,
		"architecture": pkg.Architecture,
		"sha256":       sha256sum,
		"blob_id":      blobID,
		"size":         written,
		"distribution": distribution,
		"component":    component,
	}, nil
}

func (h *DebUploadHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	packageID := vars["id"]

	repoID, err := h.resolveRepoID(r, repoName)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	// Look up component to get artifact ID for blob cleanup
	comp, err := h.db.GetDebPackageByID(r.Context(), packageID)
	if err != nil {
		h.log.Errorw("failed to look up component", "package_id", packageID, "error", err)
		writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to look up package")

		return
	}

	if comp == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "package not found")
		return
	}

	if err := h.db.DeleteComponent(r.Context(), packageID); err != nil {
		h.log.Errorw("failed to delete component", "package_id", packageID, "error", err)
		writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to delete package")

		return
	}

	// Delete blob from storage (best-effort)
	if comp.BlobID != "" {
		if err := h.blobStore.Delete(r.Context(), comp.BlobID); err != nil {
			h.log.Warnw("failed to delete blob", "blob_id", comp.BlobID, "error", err)
		}
	}

	if h.debWorker != nil {
		h.debWorker.NotifyStale(repoID)
	}

	h.log.Infow("Deb package deleted", "repository", repoName, "package_id", packageID)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]any{
			"repository": repoName,
			"package_id": packageID,
		},
	})
}
