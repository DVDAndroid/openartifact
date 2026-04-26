// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package raw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type HTTPHandler struct {
	*base.Handler
}

func NewHTTPHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *HTTPHandler {
	return &HTTPHandler{Handler: base.New(log, db, blobStore, baseURL)}
}

// RegisterRepoRoutes registers Raw-specific routes on the provided sub-router.
// It does NOT wrap in r.Route() — callers are responsible for the path prefix.
func (h *HTTPHandler) RegisterRepoRoutes(r chi.Router) {
	r.Get("/", h.handleGet)
	r.Put("/{path:*}", h.handlePut)
	r.Get("/{path:*}", h.handleGet)
	r.Head("/{path:*}", h.handleHead)
	r.Delete("/{path:*}", h.handleDelete)
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router, prefix string) {
	r.Route(prefix, func(r chi.Router) {
		h.RegisterRepoRoutes(r)
	})
	h.Log.Infow("raw handler registered", "prefix", prefix)
}

func (h *HTTPHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	path := vars["path"]

	// Normalize path - remove leading slash if present
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		http.Error(w, "path cannot be empty", http.StatusBadRequest)
		return
	}

	repoID, err := h.ResolveRepoID(r.Context(), repoName)
	if err != nil {
		h.Log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repoID == "" {
		http.NotFound(w, r)
		return
	}

	// Read body and compute SHA256
	body := io.LimitReader(r.Body, 5*1024*1024*1024) // 5GB limit
	defer func() { _ = r.Body.Close() }()

	hasher := sha256.New()
	tee := io.TeeReader(body, hasher)

	// Detect content type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from first 512 bytes
		buf := make([]byte, 512)

		n, _ := io.ReadFull(tee, buf)
		if n > 0 {
			contentType = http.DetectContentType(buf[:n])
			// Prepend the buffer back
			tee = io.MultiReader(strings.NewReader(string(buf[:n])), tee)
		}

		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Store blob
	blobID, err := h.BlobStore.Put(r.Context(), tee, map[string]string{
		"format": "raw",
		"path":   path,
	})
	if err != nil {
		h.Log.Errorw("failed to store blob", "path", path, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	// Get blob size
	size, err := h.BlobStore.GetSize(r.Context(), blobID)
	if err != nil {
		h.Log.Errorw("failed to get blob size", "blobID", blobID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Upsert in database
	artifact := &models.RawArtifact{
		ID:           uuid.New().String(),
		RepositoryID: repoID,
		Path:         path,
		ContentType:  contentType,
		SizeBytes:    size,
		SHA256:       sha256Hex,
		BlobKey:      blobID,
		Metadata:     make(map[string]any),
	}

	if err := h.DB.UpsertRawArtifact(r.Context(), artifact); err != nil {
		h.Log.Errorw("failed to upsert artifact", "path", path, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"path":   path,
		"sha256": sha256Hex,
		"size":   size,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	path := vars["path"]

	// Normalize path
	path = strings.TrimPrefix(path, "/")

	repoID, err := h.ResolveRepoID(r.Context(), repoName)
	if err != nil {
		h.Log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repoID == "" {
		http.NotFound(w, r)
		return
	}

	// If path is empty or ends with /, list files
	if path == "" || strings.HasSuffix(path, "/") {
		h.handleList(w, r, repoID, path)
		return
	}

	// Get artifact
	artifact, err := h.DB.GetRawArtifact(r.Context(), repoID, path)
	if err != nil {
		h.Log.Errorw("failed to get artifact", "path", path, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if artifact == nil {
		http.NotFound(w, r)
		return
	}

	// Stream blob
	rc, err := h.BlobStore.Get(r.Context(), artifact.BlobKey)
	if err != nil {
		h.Log.Errorw("failed to get blob", "key", artifact.BlobKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", artifact.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(artifact.SizeBytes, 10))

	w.Header().Set("X-Checksum-Sha256", artifact.SHA256)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		h.Log.Errorw("failed to write response", "path", path, "error", err)
	}
}

func (h *HTTPHandler) handleList(w http.ResponseWriter, r *http.Request, repoID, prefix string) {
	// Remove trailing slash for prefix matching
	prefix = strings.TrimSuffix(prefix, "/")

	artifacts, err := h.DB.ListRawArtifacts(r.Context(), repoID, prefix)
	if err != nil {
		h.Log.Errorw("failed to list artifacts", "prefix", prefix, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if artifacts == nil {
		artifacts = []*models.RawArtifact{}
	}

	// Build response
	var items []map[string]any
	for _, a := range artifacts {
		items = append(items, map[string]any{
			"path":         a.Path,
			"size":         a.SizeBytes,
			"content_type": a.ContentType,
			"sha256":       a.SHA256,
			"created_at":   a.CreatedAt,
			"updated_at":   a.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(items); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleHead(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	path := vars["path"]

	// Normalize path
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		http.Error(w, "path cannot be empty", http.StatusBadRequest)
		return
	}

	repoID, err := h.ResolveRepoID(r.Context(), repoName)
	if err != nil {
		h.Log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repoID == "" {
		http.NotFound(w, r)
		return
	}

	// Get artifact
	artifact, err := h.DB.GetRawArtifact(r.Context(), repoID, path)
	if err != nil {
		h.Log.Errorw("failed to get artifact", "path", path, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if artifact == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", artifact.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(artifact.SizeBytes, 10))

	w.Header().Set("X-Checksum-Sha256", artifact.SHA256)
	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	path := vars["path"]

	// Normalize path
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		http.Error(w, "path cannot be empty", http.StatusBadRequest)
		return
	}

	repoID, err := h.ResolveRepoID(r.Context(), repoName)
	if err != nil {
		h.Log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repoID == "" {
		http.NotFound(w, r)
		return
	}

	// Delete from database
	if err := h.DB.DeleteRawArtifact(r.Context(), repoID, path); err != nil {
		h.Log.Errorw("failed to delete artifact", "path", path, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
