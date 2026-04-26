// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/worker"
	"github.com/openartifact/openartifact/pkg/logger"
	pkgmaven "github.com/openartifact/openartifact/pkg/maven"
)

const MaxMavenUploadBytes int64 = 1 << 30 // 1 GiB

type MavenUploadHandler struct {
	log         *logger.Logger
	db          database.Database
	blobStore   storage.BlobStore
	reg         *metrics.Registry
	mavenWorker *worker.MavenMetadataWorker
}

func NewMavenUploadHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, reg *metrics.Registry) *MavenUploadHandler {
	return &MavenUploadHandler{log: log, db: db, blobStore: blobStore, reg: reg}
}

// SetMavenWorker wires the debounced maven-metadata.xml regeneration worker.
// Call NotifyStale(repoID, groupID, artifactID) after each successful upload.
func (h *MavenUploadHandler) SetMavenWorker(w *worker.MavenMetadataWorker) {
	h.mavenWorker = w
}

// Register mounts PUT/DELETE using a true chi wildcard ("/*"), not a
// regexp param: chi's {name:regexp} params never span "/" (the tail
// delimiter defaults to '/', so the regexp is only ever tested against a
// single path segment), so {path:.*} can never match a multi-segment GAV
// path and silently 404s every real upload/delete.
func (h *MavenUploadHandler) Register(router chi.Router) {
	router.Put("/repos/maven/{name}/*", h.handlePut)
	router.Delete("/repos/maven/{name}/*", h.handleDelete)
}

func (h *MavenUploadHandler) resolveRepoID(ctx context.Context, name string) (string, error) {
	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		return "", err
	}

	if repo == nil {
		return "", nil
	}

	return repo.ID, nil
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *MavenUploadHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

	uploadResult := "success"

	if h.reg != nil {
		defer func() { h.reg.UploadsTotal.WithLabelValues("maven", uploadResult).Inc() }()
	}

	repoID, err := h.resolveRepoID(r.Context(), repoName)
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repoID == "" {
		uploadResult = "error"

		http.NotFound(w, r)

		return
	}

	artPath := chi.URLParam(r, "*")
	filename := filepath.Base(artPath)

	checksumSuffixes := []string{".md5", ".sha1", ".sha256", ".sha512"}
	for _, suffix := range checksumSuffixes {
		if strings.HasSuffix(filename, suffix) {
			_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, MaxMavenUploadBytes))
			w.WriteHeader(http.StatusOK)

			return
		}
	}

	body := http.MaxBytesReader(w, r.Body, MaxMavenUploadBytes)

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, body); err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to read request body", "path", artPath, "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)

		return
	}

	data := buf.Bytes()

	md5Hex, sha1Hex, sha256Hex, sha512Hex, err := pkgmaven.ComputeChecksums(bytes.NewReader(data))
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to compute checksums", "path", artPath, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	blobID, err := h.blobStore.Put(r.Context(), bytes.NewReader(data), map[string]string{
		"sha256":    sha256Hex,
		"format":    "maven",
		"repo_name": repoName,
	})
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to store blob", "path", artPath, "error", err)
		http.Error(w, "failed to store blob", http.StatusInternalServerError)

		return
	}

	groupPath, artifactIDCoord, version, _, parseErr := pkgmaven.ParseRepositoryPath(artPath)
	groupID := strings.ReplaceAll(groupPath, "/", ".")

	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	if strings.HasSuffix(filename, ".asc") {
		ext = "asc"
	}

	var (
		classifier          string
		snapshotTimestamp   string
		snapshotBuildNumber int
	)

	isSnapshot := parseErr == nil && pkgmaven.IsSnapshot(version)

	if parseErr == nil && isSnapshot {
		ts, bn, _, tsErr := pkgmaven.ParseSnapshotTimestamp(filename)
		if tsErr == nil {
			snapshotTimestamp = ts
			snapshotBuildNumber = bn
			baseWithoutExt := strings.TrimSuffix(filename, "."+ext)

			prefix := artifactIDCoord + "-" + ts + "-" + strconv.Itoa(bn)
			if len(baseWithoutExt) > len(prefix) {
				classifier = strings.TrimPrefix(baseWithoutExt[len(prefix):], "-")
			}
		}
	} else if parseErr == nil {
		baseWithoutExt := strings.TrimSuffix(filename, "."+ext)

		versionPrefix := artifactIDCoord + "-" + version
		if len(baseWithoutExt) > len(versionPrefix) {
			classifier = strings.TrimPrefix(baseWithoutExt[len(versionPrefix):], "-")
		}
	}

	art := models.MavenArtifact{
		RepositoryID:        repoID,
		BlobID:              blobID,
		GroupID:             groupID,
		ArtifactCoord:       artifactIDCoord,
		Version:             version,
		Classifier:          classifier,
		Extension:           ext,
		SnapshotTimestamp:   snapshotTimestamp,
		SnapshotBuildNumber: snapshotBuildNumber,
		IsSnapshot:          isSnapshot,
		RepoPath:            artPath,
		SHA256:              sha256Hex,
		MD5:                 md5Hex,
		SHA1:                sha1Hex,
		SHA512:              sha512Hex,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	existing, err := h.db.GetMavenArtifactByPath(r.Context(), repoID, artPath)
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to check existing artifact", "path", artPath, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if existing != nil && !isSnapshot {
		if existing.SHA256 != sha256Hex {
			uploadResult = "error"

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)

			if encErr := json.NewEncoder(w).Encode(map[string]string{
				"error": "release artifact already exists with different content",
			}); encErr != nil {
				_ = encErr // response header already written
			}

			return
		}

		w.WriteHeader(http.StatusOK)

		return
	}

	if err := h.db.CreateMavenArtifact(r.Context(), &art); err != nil {
		uploadResult = "error"

		if isUniqueViolation(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)

			if encErr := json.NewEncoder(w).Encode(map[string]string{
				"error": "artifact already exists",
			}); encErr != nil {
				_ = encErr // response header already written
			}

			return
		}

		h.log.Errorw("failed to create maven artifact", "path", artPath, "error", err)

		// The blob is content-addressed and may be referenced by other
		// artifacts; only remove it from storage once no row references
		// the same sha256 (SafeDeleteBlob checks BlobReferenceCount first).
		if delErr := storage.SafeDeleteBlob(r.Context(), h.db, h.blobStore, sha256Hex); delErr != nil {
			h.log.Warnw("failed to clean up blob after DB error", "sha256", sha256Hex, "error", delErr)
		}

		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if h.mavenWorker != nil {
		h.mavenWorker.NotifyStale(repoID, groupID, artifactIDCoord)
	}

	statusCode := http.StatusCreated
	if existing != nil {
		statusCode = http.StatusOK
	}

	w.WriteHeader(statusCode)
}

func (h *MavenUploadHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

	repoID, err := h.resolveRepoID(r.Context(), repoName)
	if err != nil {
		h.log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repoID == "" {
		http.NotFound(w, r)
		return
	}

	artPath := chi.URLParam(r, "*")

	if strings.HasSuffix(artPath, "/") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	version, parseErr := parseRepositoryPathVersion(artPath)
	if parseErr != nil || version == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	art, err := h.db.GetMavenArtifactByPath(r.Context(), repoID, artPath)
	if err != nil {
		h.log.Errorw("failed to look up artifact", "path", artPath, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if art == nil {
		http.NotFound(w, r)
		return
	}

	if err := h.db.DeleteMavenArtifact(r.Context(), art.ID); err != nil {
		h.log.Errorw("failed to delete artifact", "id", art.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseRepositoryPathVersion extracts only the version from a Maven repository path.
func parseRepositoryPathVersion(path string) (string, error) {
	groupPath, artifactID, version, filename, err := pkgmaven.ParseRepositoryPath(path)
	_ = groupPath
	_ = artifactID
	_ = filename

	return version, err
}
