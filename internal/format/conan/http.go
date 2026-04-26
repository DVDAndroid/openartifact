// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package conan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

const maxConanUploadBytes = 200 << 20

type HTTPHandler struct {
	*base.Handler
}

func NewHTTPHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *HTTPHandler {
	return &HTTPHandler{Handler: base.New(log, db, blobStore, baseURL)}
}

// RegisterRepoRoutes registers Conan-specific routes on the provided sub-router.
// It does NOT wrap in r.Route() — callers are responsible for the path prefix.
func (h *HTTPHandler) RegisterRepoRoutes(r chi.Router) {
	r.Get("/v2/conans/{pkg}/{version}/{user}/{channel}/revisions", h.handleListRevisions)
	r.Get("/v2/conans/{pkg}/{version}/{user}/{channel}/revisions/{rev}/files", h.handleListFiles)
	r.Get("/v2/conans/{pkg}/{version}/{user}/{channel}/revisions/{rev}/files/{filename}", h.handleDownload)
	r.Put("/v2/conans/{pkg}/{version}/{user}/{channel}/revisions/{rev}/files/{filename}", h.handleUpload)
	r.Delete("/v2/conans/{pkg}/{version}/{user}/{channel}", h.handleDelete)
	r.Get("/v2/conans/search", h.handleSearch)
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router, prefix string) {
	r.Route(prefix, func(r chi.Router) {
		h.RegisterRepoRoutes(r)
	})
	h.Log.Infow("conan handler registered", "prefix", prefix)
}

func (h *HTTPHandler) handleListRevisions(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	pkg := vars["pkg"]
	version := vars["version"]
	user := vars["user"]
	channel := vars["channel"]

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

	userChannel := fmt.Sprintf("%s/%s", user, channel)

	conanPkg, err := h.DB.GetConanPackage(r.Context(), repoID, pkg, version, userChannel, "0")
	if err != nil {
		h.Log.Errorw("failed to get conan package", "pkg", pkg, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if conanPkg == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"revisions": map[string]any{
			"0": map[string]any{
				"timestamp": conanPkg.CreatedAt.Unix(),
			},
		},
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"files": map[string]any{
			"conanfile.py": map[string]any{
				"size": 0,
			},
			"conaninfo.txt": map[string]any{
				"size": 0,
			},
		},
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleDownload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	pkg := vars["pkg"]
	version := vars["version"]
	user := vars["user"]
	channel := vars["channel"]
	filename := vars["filename"]

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

	userChannel := fmt.Sprintf("%s/%s", user, channel)

	conanPkg, err := h.DB.GetConanPackage(r.Context(), repoID, pkg, version, userChannel, "0")
	if err != nil {
		h.Log.Errorw("failed to get conan package", "pkg", pkg, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if conanPkg == nil {
		http.NotFound(w, r)
		return
	}

	// Download blob
	blob, err := h.BlobStore.Get(r.Context(), conanPkg.BlobKey)
	if err != nil {
		h.Log.Errorw("failed to get blob", "key", conanPkg.BlobKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	defer func() { _ = blob.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, blob)
}

func (h *HTTPHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	pkg := vars["pkg"]
	version := vars["version"]
	user := vars["user"]
	channel := vars["channel"]
	filename := vars["filename"]

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

	// Read file data
	data, err := base.ReadAllLimited(r.Body, maxConanUploadBytes)
	if err != nil {
		if errors.Is(err, base.ErrContentTooLarge) {
			http.Error(w, "upload too large", http.StatusRequestEntityTooLarge)

			return
		}
		h.Log.Errorw("failed to read upload", "error", err)
		http.Error(w, "failed to read upload", http.StatusBadRequest)

		return
	}
	defer func() { _ = r.Body.Close() }()

	sha256Hash := sha256.Sum256(data)

	// Store blob
	blobKey, err := h.BlobStore.Put(r.Context(), bytes.NewReader(data), map[string]string{
		"filename": filename,
		"package":  pkg,
	})
	if err != nil {
		h.Log.Errorw("failed to store blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	userChannel := fmt.Sprintf("%s/%s", user, channel)
	conanPkg := &models.ConanPackage{
		RepositoryID:   repoID,
		Name:           pkg,
		Version:        version,
		UserChannel:    userChannel,
		RecipeRevision: "0",
		BlobKey:        blobKey,
		ConanFile:      "",
		ConanInfo:      "",
	}

	err = h.DB.CreateConanPackage(r.Context(), conanPkg)
	if err != nil {
		h.Log.Errorw("failed to create conan package", "pkg", pkg, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"id":     conanPkg.ID,
		"name":   conanPkg.Name,
		"sha256": hex.EncodeToString(sha256Hash[:]),
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	pkg := vars["pkg"]
	version := vars["version"]
	user := vars["user"]
	channel := vars["channel"]

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

	userChannel := fmt.Sprintf("%s/%s", user, channel)

	conanPkg, err := h.DB.GetConanPackage(r.Context(), repoID, pkg, version, userChannel, "0")
	if err != nil {
		h.Log.Errorw("failed to get conan package", "pkg", pkg, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if conanPkg == nil {
		http.NotFound(w, r)
		return
	}

	// Delete blob
	_ = h.BlobStore.Delete(r.Context(), conanPkg.BlobKey)

	// Delete package record
	err = h.DB.DeleteConanPackage(r.Context(), conanPkg.ID)
	if err != nil {
		h.Log.Errorw("failed to delete conan package", "id", conanPkg.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	query := r.URL.Query().Get("q")

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

	packages, err := h.DB.ListConanPackages(r.Context(), repoID)
	if err != nil {
		h.Log.Errorw("failed to list conan packages", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Filter by query if provided
	var results []*models.ConanPackage

	for _, pkg := range packages {
		if query == "" || strings.Contains(pkg.Name, query) {
			results = append(results, pkg)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"results": results,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}
