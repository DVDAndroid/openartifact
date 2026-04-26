// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package sbt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

var _ = bytes.NewReader

type HTTPHandler struct {
	*base.Handler
}

func NewHTTPHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *HTTPHandler {
	return &HTTPHandler{Handler: base.New(log, db, blobStore, baseURL)}
}

// RegisterRepoRoutes registers SBT-specific routes on the provided sub-router.
// It does NOT wrap in r.Route() — callers are responsible for the path prefix.
func (h *HTTPHandler) RegisterRepoRoutes(r chi.Router) {
	r.Get("/{path:*}", h.handleRequest)
	r.Put("/{path:*}", h.handleRequest)
	r.Delete("/{path:*}", h.handleRequest)
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router, prefix string) {
	r.Route(prefix, func(r chi.Router) {
		h.RegisterRepoRoutes(r)
	})
	h.Log.Infow("sbt handler registered", "prefix", prefix)
}

func (h *HTTPHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	filePath := vars["path"]

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

	switch r.Method {
	case http.MethodGet:
		h.handleDownload(w, r, repoID, filePath)
	case http.MethodPut:
		h.handleUpload(w, r, repoID, filePath)
	case http.MethodDelete:
		h.handleDelete(w, r, repoID, filePath)
	}
}

func (h *HTTPHandler) handleDownload(w http.ResponseWriter, r *http.Request, repoID, filePath string) {
	// Parse path: group/artifact/version/filename
	parts := strings.Split(filePath, "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}

	groupID := parts[0]
	artifactID := parts[1]
	version := parts[2]
	filename := strings.Join(parts[3:], "/")

	artifact, err := h.DB.GetSBTArtifact(r.Context(), repoID, groupID, artifactID, version, filename)
	if err != nil {
		h.Log.Errorw("failed to get sbt artifact", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if artifact == nil {
		http.NotFound(w, r)
		return
	}

	blob, err := h.BlobStore.Get(r.Context(), artifact.BlobKey)
	if err != nil {
		h.Log.Errorw("failed to get blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	defer func() { _ = blob.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(artifact.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, blob)
}

func (h *HTTPHandler) handleUpload(w http.ResponseWriter, r *http.Request, repoID, filePath string) {
	parts := strings.Split(filePath, "/")
	if len(parts) < 4 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	groupID := parts[0]
	artifactID := parts[1]
	version := parts[2]
	filename := strings.Join(parts[3:], "/")

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read upload", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	sha256Hash := sha256.Sum256(data)

	blobKey, err := h.BlobStore.Put(r.Context(), bytes.NewReader(data), map[string]string{
		"filename": filename,
		"group":    groupID,
	})
	if err != nil {
		h.Log.Errorw("failed to store blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	artifact := &models.SBTArtifact{
		RepositoryID: repoID,
		GroupID:      groupID,
		ArtifactID:   artifactID,
		Version:      version,
		ScalaVersion: "",
		Filename:     filename,
		BlobKey:      blobKey,
		SizeBytes:    int64(len(data)),
		SHA256:       hex.EncodeToString(sha256Hash[:]),
	}

	err = h.DB.CreateSBTArtifact(r.Context(), artifact)
	if err != nil {
		h.Log.Errorw("failed to create sbt artifact", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"id":     artifact.ID,
		"sha256": artifact.SHA256,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleDelete(w http.ResponseWriter, r *http.Request, repoID, filePath string) {
	parts := strings.Split(filePath, "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}

	groupID := parts[0]
	artifactID := parts[1]
	version := parts[2]
	filename := strings.Join(parts[3:], "/")

	artifact, err := h.DB.GetSBTArtifact(r.Context(), repoID, groupID, artifactID, version, filename)
	if err != nil {
		h.Log.Errorw("failed to get sbt artifact", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if artifact == nil {
		http.NotFound(w, r)
		return
	}

	_ = h.BlobStore.Delete(r.Context(), artifact.BlobKey)

	err = h.DB.DeleteSBTArtifact(r.Context(), artifact.ID)
	if err != nil {
		h.Log.Errorw("failed to delete sbt artifact", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
