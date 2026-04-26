// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package helm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	pkghelm "github.com/openartifact/openartifact/pkg/helm"
	"github.com/openartifact/openartifact/pkg/logger"
	"gopkg.in/yaml.v3"
)

const maxHelmUploadBytes = 100 << 20

type HTTPHandler struct {
	*base.Handler
}

func NewHTTPHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string) *HTTPHandler {
	return &HTTPHandler{Handler: base.New(log, db, blobStore, baseURL)}
}

func (h *HTTPHandler) RegisterRepoRoutes(r chi.Router) {
	r.Get("/index.yaml", h.handleGetIndex)
	r.Get("/charts/{filename}", h.handleDownloadChart)
	r.Post("/charts", h.handleUploadChart)
	r.Delete("/charts/{filename}", h.handleDeleteChart)
	r.Get("/charts", h.handleListCharts)
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router, prefix string) {
	r.Route(prefix, func(r chi.Router) {
		h.RegisterRepoRoutes(r)
	})
	h.Log.Infow("helm handler registered", "prefix", prefix)
}

func (h *HTTPHandler) handleGetIndex(w http.ResponseWriter, r *http.Request) {
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

	charts, err := h.DB.ListHelmCharts(r.Context(), repoID)
	if err != nil {
		h.Log.Errorw("failed to list helm charts", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Build index.yaml
	index := buildHelmIndex(charts, repoName, h.BaseURL)

	indexYAML, err := yaml.Marshal(index)
	if err != nil {
		h.Log.Errorw("failed to marshal index.yaml", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexYAML)
}

func (h *HTTPHandler) handleDownloadChart(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
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

	// Parse filename as {chartname}-{version}.tgz
	chartName, version, err := parseChartFilename(filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	chart, err := h.DB.GetHelmChart(r.Context(), repoID, chartName, version)
	if err != nil {
		h.Log.Errorw("failed to get helm chart", "chart", chartName, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if chart == nil {
		http.NotFound(w, r)
		return
	}

	rc, err := h.BlobStore.Get(r.Context(), chart.BlobKey)
	if err != nil {
		h.Log.Errorw("failed to get chart blob", "key", chart.BlobKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Length", strconv.FormatInt(chart.SizeBytes, 10))
	w.Header().Set("ETag", `"`+chart.Digest+`"`)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		h.Log.Errorw("failed to write chart response", "filename", filename, "error", err)
	}
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *HTTPHandler) handleUploadChart(w http.ResponseWriter, r *http.Request) {
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

	// Read the chart data
	chartData, err := base.ReadAllLimited(r.Body, maxHelmUploadBytes)
	if err != nil {
		if errors.Is(err, base.ErrContentTooLarge) {
			http.Error(w, "chart too large", http.StatusRequestEntityTooLarge)

			return
		}
		h.Log.Errorw("failed to read chart data", "error", err)
		http.Error(w, "failed to read chart", http.StatusBadRequest)

		return
	}
	defer func() { _ = r.Body.Close() }()

	// Parse the chart to extract metadata
	metadata, err := pkghelm.ParseChart(bytes.NewReader(chartData))
	if err != nil {
		h.Log.Errorw("failed to parse chart", "error", err)
		http.Error(w, "invalid chart", http.StatusBadRequest)

		return
	}

	// Extract metadata fields
	chartName, ok := metadata["name"].(string)
	if !ok || chartName == "" {
		http.Error(w, "chart name not found", http.StatusBadRequest)
		return
	}

	version, ok := metadata["version"].(string)
	if !ok || version == "" {
		http.Error(w, "chart version not found", http.StatusBadRequest)
		return
	}

	// Calculate digest
	digest := calculateSHA256(chartData)

	// Store in blob store
	blobKey, err := h.BlobStore.Put(r.Context(), bytes.NewReader(chartData), map[string]string{
		"filename": fmt.Sprintf("%s-%s.tgz", chartName, version),
		"chart":    chartName,
		"version":  version,
	})
	if err != nil {
		h.Log.Errorw("failed to store chart blob", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Extract optional fields
	appVersion, _ := metadata["appVersion"].(string)
	description, _ := metadata["description"].(string)
	home, _ := metadata["home"].(string)
	icon, _ := metadata["icon"].(string)

	apiVersion, _ := metadata["apiVersion"].(string)
	if apiVersion == "" {
		apiVersion = "v2"
	}

	chartType, _ := metadata["type"].(string)
	if chartType == "" {
		chartType = "application"
	}

	// Convert keywords
	var keywords []string

	if kw, ok := metadata["keywords"].([]any); ok {
		for _, k := range kw {
			if s, ok := k.(string); ok {
				keywords = append(keywords, s)
			}
		}
	}

	// Convert sources
	var sources []string

	if src, ok := metadata["sources"].([]any); ok {
		for _, s := range src {
			if str, ok := s.(string); ok {
				sources = append(sources, str)
			}
		}
	}

	// Convert maintainers
	var maintainers []map[string]string

	if maint, ok := metadata["maintainers"].([]any); ok {
		for _, m := range maint {
			if maintMap, ok := m.(map[string]any); ok {
				strMap := make(map[string]string)

				for k, v := range maintMap {
					if s, ok := v.(string); ok {
						strMap[k] = s
					}
				}

				if len(strMap) > 0 {
					maintainers = append(maintainers, strMap)
				}
			}
		}
	}

	// Create chart record
	chart := &models.HelmChart{
		RepositoryID: repoID,
		Name:         chartName,
		Version:      version,
		AppVersion:   appVersion,
		Description:  description,
		Keywords:     keywords,
		Home:         home,
		Sources:      sources,
		Maintainers:  maintainers,
		Icon:         icon,
		APIVersion:   apiVersion,
		Type:         chartType,
		BlobKey:      blobKey,
		Digest:       digest,
		SizeBytes:    int64(len(chartData)),
	}

	if err := h.DB.CreateHelmChart(r.Context(), chart); err != nil {
		h.Log.Errorw("failed to create helm chart record", "chart", chartName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"id":      chart.ID,
		"name":    chart.Name,
		"version": chart.Version,
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *HTTPHandler) handleDeleteChart(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
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

	// Parse filename
	chartName, version, err := parseChartFilename(filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get chart to find blob key
	chart, err := h.DB.GetHelmChart(r.Context(), repoID, chartName, version)
	if err != nil {
		h.Log.Errorw("failed to get helm chart", "chart", chartName, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if chart == nil {
		http.NotFound(w, r)
		return
	}

	// Delete from blob store
	if err := h.BlobStore.Delete(r.Context(), chart.BlobKey); err != nil {
		h.Log.Errorw("failed to delete chart blob", "key", chart.BlobKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Delete from database
	if err := h.DB.DeleteHelmChart(r.Context(), repoID, chartName, version); err != nil {
		h.Log.Errorw("failed to delete helm chart record", "chart", chartName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandler) handleListCharts(w http.ResponseWriter, r *http.Request) {
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

	charts, err := h.DB.ListHelmCharts(r.Context(), repoID)
	if err != nil {
		h.Log.Errorw("failed to list helm charts", "repo", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Convert to JSON-friendly format
	var result []map[string]any
	for _, chart := range charts {
		result = append(result, map[string]any{
			"id":          chart.ID,
			"name":        chart.Name,
			"version":     chart.Version,
			"appVersion":  chart.AppVersion,
			"description": chart.Description,
			"digest":      chart.Digest,
			"size":        chart.SizeBytes,
			"createdAt":   chart.CreatedAt,
			"updatedAt":   chart.UpdatedAt,
		})
	}

	if result == nil {
		result = []map[string]any{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(result); encErr != nil {
		_ = encErr // response header already written
	}
}

// Helper functions

func parseChartFilename(filename string) (string, string, error) {
	// Expected format: {name}-{version}.tgz
	if !strings.HasSuffix(filename, ".tgz") {
		return "", "", errors.New("invalid filename") //nolint:err113 // static error
	}

	// Remove .tgz extension
	base := strings.TrimSuffix(filename, ".tgz")

	// Find the last hyphen to split name and version
	name, version, found := strings.CutLast(base, "-")
	if !found {
		return "", "", errors.New("invalid filename format") //nolint:err113 // static error
	}

	if name == "" || version == "" {
		return "", "", errors.New("invalid filename format") //nolint:err113 // static error
	}

	return name, version, nil
}

func calculateSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// HelmIndex represents the Helm repository index.yaml structure.
type HelmIndex struct {
	APIVersion string                      `yaml:"apiVersion"`
	Generated  string                      `yaml:"generated"`
	Entries    map[string][]HelmChartEntry `yaml:"entries"`
}

type HelmChartEntry struct {
	Name        string              `yaml:"name"`
	Version     string              `yaml:"version"`
	AppVersion  string              `yaml:"appVersion,omitempty"`
	Description string              `yaml:"description,omitempty"`
	Digest      string              `yaml:"digest"`
	URLs        []string            `yaml:"urls"`
	Created     string              `yaml:"created"`
	Home        string              `yaml:"home,omitempty"`
	Sources     []string            `yaml:"sources,omitempty"`
	Maintainers []map[string]string `yaml:"maintainers,omitempty"`
	Icon        string              `yaml:"icon,omitempty"`
	APIVersion  string              `yaml:"apiVersion,omitempty"`
	Type        string              `yaml:"type,omitempty"`
	Keywords    []string            `yaml:"keywords,omitempty"`
}

func buildHelmIndex(charts []*models.HelmChart, repoName, baseURL string) *HelmIndex {
	entries := make(map[string][]HelmChartEntry)

	for _, chart := range charts {
		entry := HelmChartEntry{
			Name:        chart.Name,
			Version:     chart.Version,
			AppVersion:  chart.AppVersion,
			Description: chart.Description,
			Digest:      chart.Digest,
			URLs: []string{
				fmt.Sprintf("%s/repos/%s/charts/%s-%s.tgz", baseURL, repoName, chart.Name, chart.Version),
			},
			Created:     chart.CreatedAt.UTC().Format(time.RFC3339),
			Home:        chart.Home,
			Sources:     chart.Sources,
			Maintainers: chart.Maintainers,
			Icon:        chart.Icon,
			APIVersion:  chart.APIVersion,
			Type:        chart.Type,
			Keywords:    chart.Keywords,
		}

		entries[chart.Name] = append(entries[chart.Name], entry)
	}

	return &HelmIndex{
		APIVersion: "v1",
		Generated:  time.Now().UTC().Format(time.RFC3339),
		Entries:    entries,
	}
}
