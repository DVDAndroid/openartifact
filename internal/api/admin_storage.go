// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type AdminStorageHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewAdminStorageHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *AdminStorageHandler {
	return &AdminStorageHandler{db: db, log: log, evaluator: evaluator}
}

//nolint:gochecknoglobals // immutable lookup table
var validStorageKinds = map[string]bool{
	"filesystem": true,
	"s3":         true,
	"gcs":        true,
	"azure":      true,
	"garage":     true,
	"seaweedfs":  true,
	"oci":        true,
}

func (h *AdminStorageHandler) Register(r chi.Router) {
	r.Get("/api/v1/admin/storage", h.handleList)
	r.Post("/api/v1/admin/storage", h.handleCreate)
	r.Get("/api/v1/admin/storage/{name}", h.handleGet)
	r.Patch("/api/v1/admin/storage/{name}", h.handleUpdate)
	r.Delete("/api/v1/admin/storage/{name}", h.handleDelete)
	r.Post("/api/v1/admin/storage/{name}/test", h.handleTest)
}

func (h *AdminStorageHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "auth evaluation failed")
		return false
	}

	if !isAdmin {
		writeError(w, http.StatusForbidden, "admin_required", "admin access required")
		return false
	}

	return true
}

func (h *AdminStorageHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	backends, err := h.db.ListStorageBackends(r.Context())
	if err != nil {
		h.log.Error("failed to list storage backends", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list storage backends")

		return
	}

	data := make([]responses.StorageBackendResponse, len(backends))
	for i, b := range backends {
		data[i] = storageBackendToResponse(b)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(data),
		"items": data,
	})
}

func (h *AdminStorageHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var body struct {
		Name      string          `json:"name"`
		Kind      string          `json:"kind"`
		IsDefault bool            `json:"is_default"`
		Config    json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body")
		return
	}

	// Validate required fields
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	if body.Kind == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "kind is required")
		return
	}

	// Validate kind
	if !validStorageKinds[body.Kind] {
		writeError(w, http.StatusBadRequest, "validation_error", "kind must be one of: filesystem, s3, gcs, azure, garage, seaweedfs, oci")
		return
	}

	now := time.Now()
	backend := &models.StorageBackend{
		ID:        uuid.New().String(),
		Name:      body.Name,
		Kind:      body.Kind,
		IsDefault: body.IsDefault,
		Config:    body.Config,
		Status:    "unknown",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.db.CreateStorageBackend(r.Context(), backend); err != nil {
		h.log.Error("failed to create storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create storage backend")

		return
	}

	writeJSON(w, http.StatusCreated, storageBackendToResponse(backend))
}

func (h *AdminStorageHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	name := urlparams.Vars(r)["name"]

	backend, err := h.db.GetStorageBackend(r.Context(), name)
	if err != nil {
		h.log.Error("failed to get storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get storage backend")

		return
	}

	if backend == nil {
		writeError(w, http.StatusNotFound, "not_found", "storage backend not found")
		return
	}

	writeJSON(w, http.StatusOK, storageBackendToResponse(backend))
}

func (h *AdminStorageHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	name := urlparams.Vars(r)["name"]

	backend, err := h.db.GetStorageBackend(r.Context(), name)
	if err != nil {
		h.log.Error("failed to get storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get storage backend")

		return
	}

	if backend == nil {
		writeError(w, http.StatusNotFound, "not_found", "storage backend not found")
		return
	}

	var patch struct {
		Name      *string          `json:"name"`
		Kind      *string          `json:"kind"`
		IsDefault *bool            `json:"is_default"`
		Config    *json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body")
		return
	}

	// Apply non-nil fields
	if patch.Name != nil {
		backend.Name = *patch.Name
	}

	if patch.Kind != nil {
		// Validate kind if changed
		if !validStorageKinds[*patch.Kind] {
			writeError(w, http.StatusBadRequest, "validation_error", "kind must be one of: filesystem, s3, gcs, azure, garage, seaweedfs, oci")
			return
		}

		backend.Kind = *patch.Kind
	}

	if patch.IsDefault != nil {
		backend.IsDefault = *patch.IsDefault
	}

	if patch.Config != nil {
		backend.Config = *patch.Config
	}

	backend.UpdatedAt = time.Now()

	if err := h.db.UpdateStorageBackend(r.Context(), backend); err != nil {
		h.log.Error("failed to update storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update storage backend")

		return
	}

	writeJSON(w, http.StatusOK, storageBackendToResponse(backend))
}

func (h *AdminStorageHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	name := urlparams.Vars(r)["name"]

	backend, err := h.db.GetStorageBackend(r.Context(), name)
	if err != nil {
		h.log.Error("failed to get storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get storage backend")

		return
	}

	if backend == nil {
		writeError(w, http.StatusNotFound, "not_found", "storage backend not found")
		return
	}

	if backend.IsDefault {
		writeError(w, http.StatusBadRequest, "cannot_delete_default", "cannot delete the default storage backend")
		return
	}

	if err := h.db.DeleteStorageBackend(r.Context(), backend.ID); err != nil {
		h.log.Error("failed to delete storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete storage backend")

		return
	}

	writeNoContent(w)
}

func (h *AdminStorageHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	name := urlparams.Vars(r)["name"]

	backend, err := h.db.GetStorageBackend(r.Context(), name)
	if err != nil {
		h.log.Error("failed to get storage backend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get storage backend")

		return
	}

	if backend == nil {
		writeError(w, http.StatusNotFound, "not_found", "storage backend not found")
		return
	}

	// Test based on kind
	var (
		testOk  bool
		testErr string
		testMsg string
	)

	switch backend.Kind {
	case "filesystem":
		testOk, testErr, testMsg = h.testFilesystemBackend(backend)
	case "s3", "garage", "seaweedfs":
		testOk, testErr, testMsg = h.testS3Backend(backend)
	case "oci":
		testOk, testErr, testMsg = h.testOCIBackend(backend)
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"error":   "not_implemented",
			"message": "testing for this backend type is not yet supported",
		})

		return
	}

	// Update status based on test result
	if testOk {
		if err := h.db.UpdateStorageBackendStatus(r.Context(), backend.ID, "healthy", nil); err != nil {
			h.log.Error("failed to update storage backend status", "error", err)
		}
	} else {
		if err := h.db.UpdateStorageBackendStatus(r.Context(), backend.ID, "unreachable", &testMsg); err != nil {
			h.log.Error("failed to update storage backend status", "error", err)
		}
	}

	response := map[string]any{
		"ok": testOk,
	}
	if !testOk {
		response["error"] = testErr
		response["message"] = testMsg
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *AdminStorageHandler) testFilesystemBackend(backend *models.StorageBackend) (bool, string, string) {
	var config struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(backend.Config, &config); err != nil {
		return false, "invalid_config", "failed to parse config: " + err.Error()
	}

	if config.Path == "" {
		return false, "invalid_config", "path is required in config"
	}

	// Check if path exists
	if _, err := os.Stat(config.Path); err != nil {
		return false, "path_not_found", "storage path does not exist: " + err.Error()
	}

	// Test writability by creating a temp file
	tmpFile, err := os.CreateTemp(config.Path, ".test-*")
	if err != nil {
		return false, "not_writable", "storage path is not writable: " + err.Error()
	}

	// Clean up temp file
	_ = tmpFile.Close()
	_ = os.Remove(tmpFile.Name())

	return true, "", ""
}

func (h *AdminStorageHandler) testS3Backend(backend *models.StorageBackend) (bool, string, string) {
	var config struct {
		Endpoint  string `json:"endpoint"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
	}

	if err := json.Unmarshal(backend.Config, &config); err != nil {
		return false, "invalid_config", "failed to parse config: " + err.Error()
	}

	if config.Endpoint == "" {
		return false, "invalid_config", "endpoint is required in config"
	}

	if config.AccessKey == "" || config.SecretKey == "" {
		return false, "invalid_config", "access_key and secret_key are required in config"
	}

	if config.Bucket == "" {
		return false, "invalid_config", "bucket is required in config"
	}

	// Test connectivity by making a simple HEAD request to the endpoint
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, config.Endpoint, nil)
	if err != nil {
		return false, "connection_failed", "failed to create request: " + err.Error()
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, "connection_failed", "failed to connect to endpoint: " + err.Error()
	}

	defer func() { _ = resp.Body.Close() }()

	// Accept any response status (endpoint may require auth for actual operations)
	return true, "", ""
}

func (h *AdminStorageHandler) testOCIBackend(backend *models.StorageBackend) (bool, string, string) {
	var config struct {
		Endpoint string `json:"endpoint"`
	}

	if err := json.Unmarshal(backend.Config, &config); err != nil {
		return false, "invalid_config", "failed to parse config: " + err.Error()
	}

	if config.Endpoint == "" {
		return false, "invalid_config", "endpoint is required in config"
	}

	// Test OCI registry connectivity with GET /v2/ request
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, config.Endpoint+"/v2/", nil)
	if err != nil {
		return false, "connection_failed", "failed to create request: " + err.Error()
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, "connection_failed", "failed to connect to OCI endpoint: " + err.Error()
	}

	defer func() { _ = resp.Body.Close() }()

	// OCI registries should respond with 200 or 401 (if auth required) to /v2/
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return false, "invalid_endpoint", "endpoint does not appear to be a valid OCI registry"
	}

	return true, "", ""
}

func storageBackendToResponse(b *models.StorageBackend) responses.StorageBackendResponse {
	var config map[string]any
	if len(b.Config) > 0 {
		_ = json.Unmarshal(b.Config, &config)
	}

	return responses.StorageBackendResponse{
		ID:            b.ID,
		Name:          b.Name,
		Kind:          b.Kind,
		IsDefault:     b.IsDefault,
		Config:        config,
		Status:        b.Status,
		StatusMessage: b.StatusMessage,
		LastCheckedAt: formatTimePtr(b.LastCheckedAt),
		CreatedAt:     formatTime(b.CreatedAt),
		UpdatedAt:     formatTime(b.UpdatedAt),
	}
}
