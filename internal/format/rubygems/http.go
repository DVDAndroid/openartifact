// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rubygems

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
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
	"github.com/openartifact/openartifact/pkg/rubymarshal"
)

type HTTPHandler struct {
	*base.Handler
}

func NewHTTPHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *HTTPHandler {
	return &HTTPHandler{Handler: base.New(log, db, blobStore, baseURL)}
}

// RegisterRepoRoutes registers RubyGems-specific routes on the provided sub-router.
// It does NOT wrap in r.Route() — callers are responsible for the path prefix.
func (h *HTTPHandler) RegisterRepoRoutes(r chi.Router) {
	r.Get("/specs.4.8.gz", h.handleSpecsGz)
	r.Get("/latest_specs.4.8.gz", h.handleLatestSpecsGz)
	r.Get("/prerelease_specs.4.8.gz", h.handlePrereleaseSpecsGz)
	r.Get("/quick/Marshal.4.8/{filename}", h.handleGemspec)
	r.Get("/gems/{filename}", h.handleGemDownload)
	r.Post("/api/v1/gems", h.handleUploadGem)
	r.Delete("/api/v1/gems/yank", h.handleYankGem)
	r.Get("/api/v1/gems/{name}.json", h.handleGemInfo)
	r.Get("/api/v1/search.json", h.handleSearch)
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router, prefix string) {
	r.Route(prefix, func(r chi.Router) {
		h.RegisterRepoRoutes(r)
	})
	h.Log.Infow("rubygems handler registered", "prefix", prefix)
}

// handleSpecsGz returns all gem specs as gzipped Marshal data.
func (h *HTTPHandler) handleSpecsGz(w http.ResponseWriter, r *http.Request) {
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

	gems, err := h.DB.ListRubyGems(r.Context(), repoID)
	if err != nil {
		h.Log.Errorw("failed to list gems", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Filter out yanked gems
	var activeGems []*models.RubyGem

	for _, g := range gems {
		if !g.Yanked {
			activeGems = append(activeGems, g)
		}
	}

	marshalData, err := marshalSpecsList(activeGems)
	if err != nil {
		h.Log.Errorw("failed to marshal gem specs", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	gzw := gzip.NewWriter(w)
	defer func() { _ = gzw.Close() }()

	_, _ = gzw.Write(marshalData)
}

// handleLatestSpecsGz returns only latest versions as gzipped Marshal data.
func (h *HTTPHandler) handleLatestSpecsGz(w http.ResponseWriter, r *http.Request) {
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

	gems, err := h.DB.ListRubyGems(r.Context(), repoID)
	if err != nil {
		h.Log.Errorw("failed to list gems", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Filter to only latest versions per gem name
	latestByName := make(map[string]*models.RubyGem)

	for _, g := range gems {
		if g.Yanked {
			continue
		}

		key := g.Name + ":" + g.Platform
		if existing, ok := latestByName[key]; ok {
			// Keep the one with higher version (assuming semantic versioning)
			if compareVersions(g.Version, existing.Version) > 0 {
				latestByName[key] = g
			}
		} else {
			latestByName[key] = g
		}
	}

	var latestGems []*models.RubyGem
	for _, g := range latestByName {
		latestGems = append(latestGems, g)
	}

	marshalData, err := marshalSpecsList(latestGems)
	if err != nil {
		h.Log.Errorw("failed to marshal gem specs", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	gzw := gzip.NewWriter(w)
	defer func() { _ = gzw.Close() }()

	_, _ = gzw.Write(marshalData)
}

// handlePrereleaseSpecsGz returns prerelease gems as gzipped Marshal data.
func (h *HTTPHandler) handlePrereleaseSpecsGz(w http.ResponseWriter, r *http.Request) {
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

	gems, err := h.DB.ListRubyGems(r.Context(), repoID)
	if err != nil {
		h.Log.Errorw("failed to list gems", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Filter to prerelease versions (contain alpha, beta, rc, etc.)
	var prereleaseGems []*models.RubyGem

	for _, g := range gems {
		if !g.Yanked && isPrerelease(g.Version) {
			prereleaseGems = append(prereleaseGems, g)
		}
	}

	marshalData, err := marshalSpecsList(prereleaseGems)
	if err != nil {
		h.Log.Errorw("failed to marshal gem specs", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	gzw := gzip.NewWriter(w)
	defer func() { _ = gzw.Close() }()

	_, _ = gzw.Write(marshalData)
}

// handleGemspec returns a zlib-compressed gemspec.
func (h *HTTPHandler) handleGemspec(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	filename := vars["filename"]

	// Parse filename to extract gemname and version
	// Format: gemname-version.gemspec.rz
	// Strip the .gemspec.rz suffix
	const suffix = ".gemspec.rz"
	if !strings.HasSuffix(filename, suffix) {
		http.NotFound(w, r)
		return
	}

	nameVersion := strings.TrimSuffix(filename, suffix)
	// Split on the last hyphen to separate name and version
	gemname, version, found := strings.CutLast(nameVersion, "-")
	if !found {
		http.NotFound(w, r)
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

	gem, err := h.DB.GetRubyGem(r.Context(), repoID, gemname, version, "ruby")
	if err != nil {
		h.Log.Errorw("failed to get gem", "name", gemname, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if gem == nil {
		http.NotFound(w, r)
		return
	}

	spec := rubymarshal.Specification{
		Name:         gem.Name,
		Version:      gem.Version,
		Platform:     gem.Platform,
		Summary:      gem.Summary,
		Description:  gem.Description,
		Homepage:     gem.Homepage,
		Authors:      gem.Authors,
		Licenses:     gem.Licenses,
		Dependencies: decodeGemDependencies(gem.Dependencies),
	}

	data, err := rubymarshal.EncodeSpecification(spec)
	if err != nil {
		h.Log.Errorw("failed to encode gemspec", "name", gemname, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	zw := zlib.NewWriter(w)
	defer func() { _ = zw.Close() }()

	_, _ = zw.Write(data)
}

// handleGemDownload serves the .gem file.
func (h *HTTPHandler) handleGemDownload(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	filename := vars["filename"]

	// Parse filename to extract gemname and version
	// Format: gemname-version.gem
	const suffix = ".gem"
	if !strings.HasSuffix(filename, suffix) {
		http.NotFound(w, r)
		return
	}

	nameVersion := strings.TrimSuffix(filename, suffix)
	// Split on the last hyphen to separate name and version
	gemname, version, found := strings.CutLast(nameVersion, "-")
	if !found {
		http.NotFound(w, r)
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

	gem, err := h.DB.GetRubyGem(r.Context(), repoID, gemname, version, "ruby")
	if err != nil {
		h.Log.Errorw("failed to get gem", "name", gemname, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if gem == nil {
		http.NotFound(w, r)
		return
	}

	// Retrieve blob from storage
	blob, err := h.BlobStore.Get(r.Context(), gem.BlobKey)
	if err != nil {
		h.Log.Errorw("failed to get blob", "key", gem.BlobKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	defer func() { _ = blob.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.gem"`, gemname, version))
	_, _ = io.Copy(w, blob)
}

// handleUploadGem handles gem uploads.
func (h *HTTPHandler) handleUploadGem(w http.ResponseWriter, r *http.Request) {
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

	// Read gem file from request body
	gemData, err := io.ReadAll(r.Body)
	if err != nil {
		h.Log.Errorw("failed to read request body", "error", err)
		http.Error(w, "failed to read gem", http.StatusBadRequest)

		return
	}
	defer func() { _ = r.Body.Close() }()

	// Parse gem metadata
	handler := NewRubyGemsHandler()

	metadata, err := handler.Parse(r.Context(), bytes.NewReader(gemData))
	if err != nil {
		h.Log.Errorw("failed to parse gem", "error", err)
		http.Error(w, "invalid gem file", http.StatusBadRequest)

		return
	}

	// Extract gem name and version from metadata
	name, ok := metadata["name"].(string)
	if !ok {
		name = "unknown"
	}

	version, ok := metadata["version"].(string)
	if !ok {
		version = "0.0.0"
	}

	platform := getStringFromMetadata(metadata, "platform")
	if platform == "" {
		platform = "ruby"
	}

	// Store blob
	blobKey, err := h.BlobStore.Put(r.Context(), bytes.NewReader(gemData), map[string]string{
		"gem_name": name,
		"version":  version,
	})
	if err != nil {
		h.Log.Errorw("failed to store blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Create database record
	gem := &models.RubyGem{
		RepositoryID:  repoID,
		Name:          name,
		Version:       version,
		Platform:      platform,
		Summary:       getStringFromMetadata(metadata, "summary"),
		Description:   getStringFromMetadata(metadata, "description"),
		Homepage:      getStringFromMetadata(metadata, "homepage"),
		Authors:       getStringArrayFromMetadata(metadata, "authors"),
		Licenses:      getStringArrayFromMetadata(metadata, "licenses"),
		Dependencies:  mustMarshalJSON(metadata["dependencies"]),
		BlobKey:       blobKey,
		MetadataJSON:  mustMarshalJSON(metadata),
		DownloadCount: 0,
	}

	if err := h.DB.CreateRubyGem(r.Context(), gem); err != nil {
		h.Log.Errorw("failed to create gem record", "name", name, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"name":    gem.Name,
		"version": gem.Version,
		"message": "Gem uploaded successfully",
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

// handleYankGem yanks a gem version.
func (h *HTTPHandler) handleYankGem(w http.ResponseWriter, r *http.Request) {
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

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	gemName := r.FormValue("gem_name")
	version := r.FormValue("version")

	if gemName == "" || version == "" {
		http.Error(w, "gem_name and version are required", http.StatusBadRequest)
		return
	}

	if err := h.DB.YankRubyGem(r.Context(), repoID, gemName, version, true); err != nil {
		h.Log.Errorw("failed to yank gem", "name", gemName, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"message": "Gem yanked successfully",
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

// handleGemInfo returns gem information as JSON.
func (h *HTTPHandler) handleGemInfo(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	gemName := vars["name"]

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

	versions, err := h.DB.ListRubyGemVersions(r.Context(), repoID, gemName)
	if err != nil {
		h.Log.Errorw("failed to list gem versions", "name", gemName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if len(versions) == 0 {
		http.NotFound(w, r)
		return
	}

	// Build response
	response := map[string]any{
		"name":     gemName,
		"versions": versions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
		_ = encErr // response header already written
	}
}

// handleSearch searches for gems.
func (h *HTTPHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
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

	query := r.URL.Query().Get("query")
	if query == "" {
		query = "*"
	}

	gems, err := h.DB.SearchRubyGems(r.Context(), repoID, query)
	if err != nil {
		h.Log.Errorw("failed to search gems", "query", query, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"gems": gems,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

// Helper functions

func getStringFromMetadata(metadata map[string]any, key string) string {
	if v, ok := metadata[key].(string); ok {
		return v
	}

	return ""
}

func getStringArrayFromMetadata(metadata map[string]any, key string) []string {
	if v, ok := metadata[key].([]any); ok {
		var result []string

		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}

		return result
	}

	return nil
}

func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		// Return empty JSON object on error
		return []byte("{}")
	}

	return data
}

func isPrerelease(version string) bool {
	lower := strings.ToLower(version)

	return strings.Contains(lower, "alpha") || strings.Contains(lower, "beta") ||
		strings.Contains(lower, "rc") || strings.Contains(lower, "pre") ||
		strings.Contains(lower, "dev")
}

func compareVersions(v1, v2 string) int {
	// Lexicographic string comparison.
	if v1 > v2 {
		return 1
	} else if v1 < v2 {
		return -1
	}

	return 0
}

// marshalSpecsList creates a Ruby Marshal 4.8 encoded array of gem specs:
// [[name, Gem::Version, platform], ...], the format specs.4.8.gz et al.
// serve. Routed through pkg/rubymarshal instead of hand-written byte
// twiddling so it shares one codec with handleGemspec.
func marshalSpecsList(gems []*models.RubyGem) ([]byte, error) {
	entries := make([]any, len(gems))

	for i, gem := range gems {
		entries[i] = []any{gem.Name, rubymarshal.NewGemVersion(gem.Version), gem.Platform}
	}

	data, err := rubymarshal.Encode(entries)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal specs list: %w", err)
	}

	return data, nil
}

// gemDependencyJSON is the {"name","requirement","type"} shape
// parseGemspecMetadata/handleUploadGem persist into RubyGem.Dependencies.
type gemDependencyJSON struct {
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
	Type        string `json:"type"`
}

// decodeGemDependencies parses a RubyGem.Dependencies JSON column back into
// rubymarshal.Dependency values for re-encoding into a served gemspec.
func decodeGemDependencies(raw []byte) []rubymarshal.Dependency {
	if len(raw) == 0 {
		return nil
	}

	var deps []gemDependencyJSON
	if err := json.Unmarshal(raw, &deps); err != nil {
		return nil
	}

	out := make([]rubymarshal.Dependency, len(deps))
	for i, d := range deps {
		out[i] = rubymarshal.Dependency{Name: d.Name, Requirement: d.Requirement, Type: d.Type}
	}

	return out
}
