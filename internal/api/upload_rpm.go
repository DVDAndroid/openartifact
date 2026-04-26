// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	database "github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	formatrpm "github.com/openartifact/openartifact/internal/format/rpm"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/worker"
	"github.com/openartifact/openartifact/pkg/logger"
	pkgrpm "github.com/openartifact/openartifact/pkg/rpm"
)

const maxRPMUploadSize = 500 * 1024 * 1024 // 500MB

type RPMUploadHandler struct {
	log        *logger.Logger
	db         database.Database
	blobStore  storage.BlobStore
	handler    *formatrpm.RepositoryHandler
	reg        *metrics.Registry
	debHandler *DebUploadHandler
	rpmWorker  *worker.RPMetadataWorker
}

func NewRPMUploadHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, reg *metrics.Registry) *RPMUploadHandler {
	return &RPMUploadHandler{
		log:        log,
		db:         db,
		blobStore:  blobStore,
		handler:    formatrpm.NewRepositoryHandler(),
		reg:        reg,
		debHandler: NewDebUploadHandler(log, db, blobStore, reg),
	}
}

func (h *RPMUploadHandler) SetDebWorker(w *worker.DebMetadataWorker) {
	h.debHandler.SetDebWorker(w)
}

func (h *RPMUploadHandler) SetRPMWorker(w *worker.RPMetadataWorker) {
	h.rpmWorker = w
}

func (h *RPMUploadHandler) Register(router chi.Router) {
	router.Post("/api/v1/repositories/{name}/packages", h.handleUpload)
	router.Delete("/api/v1/repositories/{name}/packages/{id}", h.handleDelete)
	router.Put("/api/v1/repositories/{name}/comps", h.handleCompsUpload)
	h.log.Infow("RPM upload handler registered")
}

func (h *RPMUploadHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

	// Dispatch to the correct format handler based on the repository's format.
	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	if repo.Format == "deb" {
		h.debHandler.handleUpload(w, r)
		return
	}

	uploadResult := "success"

	if h.reg != nil {
		defer func() { h.reg.UploadsTotal.WithLabelValues("rpm", uploadResult).Inc() }()
	}

	if err := r.ParseMultipartForm(maxRPMUploadSize); err != nil { //nolint:gosec // G120: form size is bounded by maxRPMUploadSize
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

	if header.Size > maxRPMUploadSize {
		uploadResult = "error"

		writeError(w, http.StatusRequestEntityTooLarge, "INVALID_INPUT", "file exceeds maximum size limit")

		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/x-rpm" && contentType != "application/octet-stream" {
		uploadResult = "error"

		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid content type: "+contentType)

		return
	}

	result, err := h.uploadPipeline(r, file, repoName)
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("RPM upload failed", "repository", repoName, "filename", header.Filename, "error", err)
		writeError(w, http.StatusInternalServerError, "UPLOAD_ERROR", "upload processing failed")

		return
	}

	if result.Duplicate {
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

	h.log.Infow("RPM package uploaded", "repository", repoName, "package", result.Name, "version", result.Version)
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    result,
	})
}

func (h *RPMUploadHandler) resolveRepoID(r *http.Request, repoName string) (string, error) {
	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil {
		return "", fmt.Errorf("failed to look up repository: %w", err)
	}

	if repo == nil {
		return "", fmt.Errorf("repository not found: %s", repoName) //nolint:err113 // dynamic error with runtime context
	}

	return repo.ID, nil
}

// ensureRPMRepo makes sure the rpm_repos row the metadata worker needs exists.
func (h *RPMUploadHandler) ensureRPMRepo(ctx context.Context, repoID string) error {
	rpmRepo, err := h.db.GetRPMRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to check rpm_repos: %w", err)
	}

	if rpmRepo == nil {
		if err := h.db.CreateRPMRepo(ctx, &models.RPMRepo{RepositoryID: repoID, BaseArch: "x86_64"}); err != nil {
			return fmt.Errorf("failed to create rpm_repos entry: %w", err)
		}
	}

	return nil
}

// storeBlobDeduped stores the temp file in the blob store unless a blob with
// the same sha256 already exists, and reports whether it pre-existed.
func (h *RPMUploadHandler) storeBlobDeduped(ctx context.Context, tmpFile *os.File, repoName, sha256sum string) (string, bool, error) {
	blobID, blobExists, err := h.blobStore.ExistsBySHA256(ctx, sha256sum)
	if err != nil {
		return "", false, fmt.Errorf("failed to check blob existence: %w", err)
	}

	if blobExists {
		return blobID, true, nil
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", false, fmt.Errorf("failed to seek temp file: %w", err)
	}

	blobID, err = h.blobStore.Put(ctx, tmpFile, map[string]string{
		"format":    "rpm",
		"repo_name": repoName,
		"sha256":    sha256sum,
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to store blob: %w", err)
	}

	return blobID, false, nil
}

func (h *RPMUploadHandler) uploadPipeline(r *http.Request, file io.Reader, repoName string) (*formatrpm.UploadResult, error) {
	repoID, err := h.resolveRepoID(r, repoName)
	if err != nil {
		return nil, err
	}

	if err := h.ensureRPMRepo(r.Context(), repoID); err != nil {
		return nil, err
	}

	// Stream to temp file while computing SHA256
	tmpFile, err := os.CreateTemp("", "rpm-upload-*.tmp")
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
		return &formatrpm.UploadResult{Duplicate: true}, nil
	}

	// Parse from temp file
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	pkg, err := pkgrpm.ParseRPM(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rpm: %w", err)
	}

	filename := fmt.Sprintf("%s-%s-%s.%s.rpm", pkg.Name, pkg.Version, pkg.Release, pkg.Arch)

	// Cross-repo dedup: if the blob already exists in any repo, skip re-uploading it.
	// The component record is still created for this repo, pointing to the shared blob.
	blobID, blobExists, err := h.storeBlobDeduped(r.Context(), tmpFile, repoName, sha256sum)
	if err != nil {
		return nil, err
	}

	// Upsert into blobs table to get the UUID for blob_id FK reference.
	blobUUID, err := h.db.UpsertBlob(r.Context(), sha256sum, sha256sum, written)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert blob record: %w", err)
	}

	component := &models.Component{
		RepositoryID:  repoID,
		ArtifactID:    blobUUID,
		Name:          pkg.Name,
		Version:       pkg.Version,
		Release:       pkg.Release,
		Epoch:         int(pkg.Epoch),
		Arch:          pkg.Arch,
		Summary:       pkg.Summary,
		Description:   pkg.Description,
		License:       pkg.License,
		URL:           pkg.URL,
		Vendor:        pkg.Vendor,
		Group:         pkg.Group,
		Packager:      pkg.Packager,
		SourceRPM:     pkg.SourceRPM,
		BuildHost:     pkg.BuildHost,
		BuildTime:     pkg.BuildTime,
		PackageSize:   written,
		InstalledSize: pkg.InstalledSize,
		ArchiveSize:   pkg.ArchiveSize,
		Filename:      filename,
	}

	if err := h.db.CreateRpmPackage(r.Context(), component); err != nil {
		// Lost the check-then-insert race: another concurrent upload of the
		// same content won. The shared blob is referenced by the winner, so
		// do not delete it — just report the duplicate (HTTP 409).
		if errors.Is(err, database.ErrDuplicate) {
			return &formatrpm.UploadResult{Duplicate: true}, nil
		}

		if !blobExists {
			if delErr := h.blobStore.Delete(r.Context(), blobID); delErr != nil {
				h.log.Warnw("failed to clean up blob after DB error", "blob_id", blobID, "error", delErr)
			}
		}
		return nil, fmt.Errorf("failed to create component record: %w", err)
	}
	if h.rpmWorker != nil {
		h.rpmWorker.NotifyStale(repoID)
	}

	return &formatrpm.UploadResult{
		Name:    pkg.Name,
		Version: pkg.Version,
		Release: pkg.Release,
		Arch:    pkg.Arch,
		SHA256:  sha256sum,
		BlobID:  blobID,
		Size:    written,
	}, nil
}

func (h *RPMUploadHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	packageID := vars["id"]

	// Dispatch to deb handler for deb repositories
	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	if repo.Format == "deb" {
		h.debHandler.handleDelete(w, r)
		return
	}

	repoID, err := h.resolveRepoID(r, repoName)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	// Look up component to get artifact ID for blob cleanup
	comp, err := h.db.GetRpmPackageByID(r.Context(), packageID)
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

	// Delete blob from storage (best-effort); blob is keyed by BlobID
	if comp.BlobID != "" {
		if err := h.blobStore.Delete(r.Context(), comp.BlobID); err != nil {
			h.log.Warnw("failed to delete blob", "blob_id", comp.BlobID, "error", err)
		}
	}

	if err := h.db.MarkRepoStale(r.Context(), repoID); err != nil {
		h.log.Warnw("failed to mark repo stale after delete", "repo_id", repoID, "error", err)
	}

	if h.rpmWorker != nil {
		h.rpmWorker.NotifyStale(repoID)
	}

	h.log.Infow("RPM package deleted", "repository", repoName, "package_id", packageID)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]any{
			"repository": repoName,
			"package_id": packageID,
		},
	})
}

func (h *RPMUploadHandler) handleCompsUpload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

	repoID, err := h.resolveRepoID(r, repoName)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "repository not found: "+repoName)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "failed to read request body")
		return
	}

	if err := formatrpm.ValidateCompsXML(body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", fmt.Sprintf("invalid comps XML: %v", err))
		return
	}

	compsSHA := fmt.Sprintf("%x", sha256.Sum256(body))

	blobID, err := h.blobStore.Put(r.Context(), bytes.NewReader(body), map[string]string{
		"format":    "rpm",
		"type":      "comps",
		"repo_name": repoName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to store comps XML")
		return
	}

	meta := &models.RepoMetadata{
		RepositoryID: repoID,
		Path:         "comps.xml",
		BlobSHA256:   compsSHA,
		BlobSize:     int64(len(body)),
	}
	if err := h.db.UpsertRepoMetadata(r.Context(), meta); err != nil {
		h.log.Errorw("failed to upsert comps metadata", "error", err)
		writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to store comps metadata")

		return
	}

	if err := h.db.MarkRepoStale(r.Context(), repoID); err != nil {
		h.log.Warnw("failed to mark repo stale after comps upload", "repo_id", repoID, "error", err)
	}

	if h.rpmWorker != nil {
		h.rpmWorker.NotifyStale(repoID)
	}

	h.log.Infow("RPM comps XML uploaded", "repository", repoName, "blob_id", blobID)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]any{
			"repository": repoName,
			"blob_id":    blobID,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if encErr := json.NewEncoder(w).Encode(v); encErr != nil {
		_ = encErr // response header already written
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if encErr := json.NewEncoder(w).Encode(ErrorResponse{
		Error: message,
		Code:  code,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}
