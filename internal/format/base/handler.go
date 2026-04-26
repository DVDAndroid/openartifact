// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package base provides shared HTTP handler infrastructure for artifact format handlers.
// Artifact-level format handlers (cargo, helm, pypi, nuget, etc.) embed BaseHandler
// to eliminate duplicated struct definitions, constructors, and resolveRepoID.
package base

import (
	"context"
	"io"
	"net/http"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/logger"
)

// Handler provides shared fields and methods for artifact-level format HTTP handlers.
// Embed this in format-specific HTTPHandler structs to eliminate duplicated boilerplate.
type Handler struct {
	Log       *logger.Logger
	DB        database.Database
	BlobStore storage.BlobStore
	BaseURL   string
}

// New creates a new base handler with the common dependencies.
func New(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *Handler {
	return &Handler{
		Log:       log,
		DB:        db,
		BlobStore: blobStore,
		BaseURL:   baseURL,
	}
}

// ResolveRepoID resolves a repository name to its ID. Returns empty string if not found.
func (h *Handler) ResolveRepoID(ctx context.Context, name string) (string, error) {
	repo, err := h.DB.GetRepositoryByName(ctx, name)
	if err != nil {
		return "", err
	}
	if repo == nil {
		return "", nil
	}
	return repo.ID, nil
}

// StreamBlob streams a blob from the store to the response writer.
// Returns false if the blob is not found.
func (h *Handler) StreamBlob(w http.ResponseWriter, r *http.Request, blobID, contentType string) bool {
	if blobID == "" {
		return false
	}
	rc, err := h.BlobStore.Get(r.Context(), blobID)
	if err != nil {
		return false
	}
	defer func() { CloseLog(rc, h.Log) }()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, err = io.Copy(w, rc)
	return err == nil
}

// CloseLog closes a Closer and logs any error. Use for deferred closes where
// the error cannot be meaningfully handled but should not be silently discarded.
func CloseLog(c io.Closer, log *logger.Logger) {
	if err := c.Close(); err != nil {
		log.Warnw("close error", "error", err)
	}
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
