// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package gomod

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

const maxGoModUploadBytes = 100 << 20

type HTTPHandler struct {
	*base.Handler
}

func NewHTTPHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *HTTPHandler {
	return &HTTPHandler{Handler: base.New(log, db, blobStore, baseURL)}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Get("/repos/{name}/{module:*}/@v/list", h.handleList)
	r.Get("/repos/{name}/{module:*}/@v/{version}.info", h.handleInfo)
	r.Get("/repos/{name}/{module:*}/@v/{version}.mod", h.handleMod)
	r.Get("/repos/{name}/{module:*}/@v/{version}.zip", h.handleZip)
	r.Get("/repos/{name}/{module:*}/@latest", h.handleLatest)
	r.Put("/repos/{name}/{module:*}/@v/{version}", h.handleUpload)
}

// decodePath decodes module path: "github.com/!my!org/pkg" -> "github.com/MyOrg/pkg".
func decodePath(encoded string) string {
	var result strings.Builder

	i := 0
	for i < len(encoded) {
		if encoded[i] == '!' && i+1 < len(encoded) {
			// Next character should be lowercase, convert to uppercase
			result.WriteString(strings.ToUpper(string(encoded[i+1])))
			i += 2
		} else {
			result.WriteString(string(encoded[i]))
			i++
		}
	}

	return result.String()
}

func (h *HTTPHandler) handleList(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	encodedModule := vars["module"]
	modulePath := decodePath(encodedModule)

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

	versions, err := h.DB.ListGoModuleVersions(r.Context(), repoID, modulePath)
	if err != nil {
		h.Log.Errorw("failed to list go module versions", "module", modulePath, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if len(versions) == 0 {
		http.NotFound(w, r)
		return
	}

	// Return newline-separated list of versions
	var versionList []string
	for _, v := range versions {
		versionList = append(versionList, v.Version)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // G705: Go module content, not HTML output
	_, _ = w.Write([]byte(strings.Join(versionList, "\n")))
}

type versionInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

func (h *HTTPHandler) handleInfo(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	encodedModule := vars["module"]
	version := vars["version"]
	modulePath := decodePath(encodedModule)

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

	mod, err := h.DB.GetGoModule(r.Context(), repoID, modulePath, version)
	if err != nil {
		h.Log.Errorw("failed to get go module", "module", modulePath, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if mod == nil {
		http.NotFound(w, r)
		return
	}

	info := versionInfo{
		Version: mod.Version,
		Time:    mod.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(info); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleMod(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	encodedModule := vars["module"]
	version := vars["version"]
	modulePath := decodePath(encodedModule)

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

	mod, err := h.DB.GetGoModule(r.Context(), repoID, modulePath, version)
	if err != nil {
		h.Log.Errorw("failed to get go module", "module", modulePath, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if mod == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // G705: Go module content, not HTML output
	_, _ = w.Write([]byte(mod.GoModContent))
}

func (h *HTTPHandler) handleZip(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	encodedModule := vars["module"]
	version := vars["version"]
	modulePath := decodePath(encodedModule)

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

	mod, err := h.DB.GetGoModule(r.Context(), repoID, modulePath, version)
	if err != nil {
		h.Log.Errorw("failed to get go module", "module", modulePath, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if mod == nil {
		http.NotFound(w, r)
		return
	}

	rc, err := h.BlobStore.Get(r.Context(), mod.ZipBlobKey)
	if err != nil {
		h.Log.Errorw("failed to get zip blob", "key", mod.ZipBlobKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Length", strconv.FormatInt(mod.ZipSizeBytes, 10))

	if mod.ZipHash != "" {
		w.Header().Set("ETag", `"`+mod.ZipHash+`"`)
	}

	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		h.Log.Errorw("failed to write zip response", "module", modulePath, "version", version, "error", err)
	}
}

func (h *HTTPHandler) handleLatest(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	encodedModule := vars["module"]
	modulePath := decodePath(encodedModule)

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

	mod, err := h.DB.GetLatestGoModule(r.Context(), repoID, modulePath)
	if err != nil {
		h.Log.Errorw("failed to get latest go module", "module", modulePath, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if mod == nil {
		http.NotFound(w, r)
		return
	}

	info := versionInfo{
		Version: mod.Version,
		Time:    mod.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(info); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	encodedModule := vars["module"]
	version := vars["version"]
	modulePath := decodePath(encodedModule)

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

	// Read the zip file from request body
	zipData, err := base.ReadAllLimited(r.Body, maxGoModUploadBytes)
	if err != nil {
		if errors.Is(err, base.ErrContentTooLarge) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)

			return
		}
		h.Log.Errorw("failed to read upload body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)

		return
	}
	defer func() { _ = r.Body.Close() }()

	// Parse go.mod from zip
	handler := NewGoModHandler()

	metadata, err := handler.Parse(r.Context(), bytes.NewReader(zipData))
	if err != nil {
		h.Log.Errorw("failed to parse go module", "error", err)
		http.Error(w, "invalid go module", http.StatusBadRequest)

		return
	}

	goModContent, ok := metadata["go_mod"].(string)
	if !ok {
		h.Log.Errorw("failed to extract go_mod from metadata", "error", "go_mod not found or not a string")
		http.Error(w, "invalid go module", http.StatusBadRequest)

		return
	}

	// Store zip blob
	zipBlobID, err := h.BlobStore.Put(r.Context(), bytes.NewReader(zipData), map[string]string{
		"module":  modulePath,
		"version": version,
		"type":    "zip",
	})
	if err != nil {
		h.Log.Errorw("failed to store zip blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Store mod blob
	modBlobID, err := h.BlobStore.Put(r.Context(), strings.NewReader(goModContent), map[string]string{
		"module":  modulePath,
		"version": version,
		"type":    "mod",
	})
	if err != nil {
		h.Log.Errorw("failed to store mod blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Create database record
	mod := &models.GoModule{
		RepositoryID: repoID,
		ModulePath:   modulePath,
		Version:      version,
		GoModContent: goModContent,
		ZipBlobKey:   zipBlobID,
		ModBlobKey:   modBlobID,
		ZipSizeBytes: int64(len(zipData)),
	}

	if err := h.DB.CreateGoModule(r.Context(), mod); err != nil {
		h.Log.Errorw("failed to create go module record", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"version": version,
		"module":  modulePath,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}
