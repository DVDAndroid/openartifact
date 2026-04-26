// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/storage"
)

type SystemInfoHandler struct {
	version     string
	commit      string
	buildTime   string
	startTime   time.Time
	db          database.Database
	blobStore   storage.BlobStore
	storageType string
}

func NewSystemInfoHandler(version, commit, buildTime string, db database.Database, blobStore storage.BlobStore, storageType string) *SystemInfoHandler {
	return &SystemInfoHandler{
		version:     version,
		commit:      commit,
		buildTime:   buildTime,
		startTime:   time.Now(),
		db:          db,
		blobStore:   blobStore,
		storageType: storageType,
	}
}

func (h *SystemInfoHandler) RegisterPublic(r chi.Router) {
	r.Get("/api/v1/system/info", h.ServeHTTP)
}

func (h *SystemInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database status
	dbStatus := "ok"

	if h.db != nil {
		if err := h.db.Ping(r.Context()); err != nil {
			dbStatus = "error"
		}
	}

	// Check storage status
	storageStatus := "ok"
	// Try to call HealthCheck if the underlying store supports it
	type healthChecker interface {
		HealthCheck(context.Context) error
	}

	if h.blobStore != nil {
		if hc, ok := h.blobStore.(healthChecker); ok {
			if err := hc.HealthCheck(r.Context()); err != nil {
				storageStatus = "error"
			}
		}
	}

	response := map[string]any{
		"version":        h.version,
		"commit":         h.commit,
		"build_time":     h.buildTime,
		"go_version":     runtime.Version(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"uptime":         time.Since(h.startTime).String(),
		"db_status":      dbStatus,
		"storage_kind":   h.storageType,
		"storage_status": storageStatus,
	}

	if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
		_ = encErr // response header already written
	}
}
