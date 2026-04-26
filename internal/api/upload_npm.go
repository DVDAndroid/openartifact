// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	formatnpm "github.com/openartifact/openartifact/internal/format/npm"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/worker"
	"github.com/openartifact/openartifact/pkg/logger"
	pkgnpm "github.com/openartifact/openartifact/pkg/npm"
)

const MaxNPMUploadBytes int64 = 200 << 20

type NPMUploadHandler struct {
	log       *logger.Logger
	db        database.Database
	blobStore storage.BlobStore
	baseURL   string
	reg       *metrics.Registry
	npmWorker *worker.NPMMetadataWorker
}

func NewNPMUploadHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore, baseURL string, reg *metrics.Registry) *NPMUploadHandler {
	return &NPMUploadHandler{log: log, db: db, blobStore: blobStore, baseURL: baseURL, reg: reg}
}

// SetNPMWorker wires the debounced npm packument regeneration worker.
// Call NotifyStale(packageID) after each successful publish.
func (h *NPMUploadHandler) SetNPMWorker(w *worker.NPMMetadataWorker) {
	h.npmWorker = w
}

func (h *NPMUploadHandler) Register(router chi.Router) {
	router.Put("/repos/{repo}/-/package/{pkg}/dist-tags/{tag}", h.handlePutDistTag)
	router.Delete("/repos/{repo}/-/package/{pkg}/dist-tags/{tag}", h.handleDeleteDistTag)
	router.Put("/repos/{repo}/*", h.handlePublish)
	router.Delete("/repos/{repo}/*", h.handleUnpublish)
}

func (h *NPMUploadHandler) resolveRepo(ctx context.Context, name string) (*models.Repository, error) {
	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func requireAuth(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && len(auth) > len("Bearer ") {
		return true
	}

	w.Header().Set("WWW-Authenticate", `Bearer realm="openartifact"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "unauthenticated"}); encErr != nil {
		_ = encErr // response header already written
	}

	return false
}

type npmPublishBody struct {
	Name        string                     `json:"name"`
	Versions    map[string]json.RawMessage `json:"versions"`
	Attachments map[string]map[string]any  `json:"_attachments"`
	DistTags    map[string]string          `json:"dist-tags"`
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *NPMUploadHandler) handlePublish(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	uploadResult := "success"

	if h.reg != nil {
		defer func() { h.reg.UploadsTotal.WithLabelValues("npm", uploadResult).Inc() }()
	}

	vars := urlparams.Vars(r)
	repoName := vars["repo"]
	rawPkg := vars["*"]
	pkgName := npmDecodePackageName(rawPkg)

	body := http.MaxBytesReader(w, r.Body, MaxNPMUploadBytes)

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(body); err != nil {
		if err.Error() == "http: request body too large" {
			uploadResult = "error"

			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)

			return
		}

		uploadResult = "error"

		h.log.Errorw("failed to read npm publish body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)

		return
	}

	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	dec.UseNumber()

	var pub npmPublishBody
	if err := dec.Decode(&pub); err != nil {
		uploadResult = "error"

		http.Error(w, "invalid JSON", http.StatusBadRequest)

		return
	}

	if pub.Name != pkgName {
		uploadResult = "error"

		http.Error(w, "name mismatch", http.StatusBadRequest)

		return
	}

	if err := pkgnpm.ValidatePackageName(pub.Name); err != nil {
		uploadResult = "error"

		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	repo, err := h.resolveRepo(r.Context(), repoName)
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repo == nil {
		uploadResult = "error"

		http.NotFound(w, r)

		return
	}

	pkg, err := h.db.GetNPMPackageByName(r.Context(), repo.ID, pub.Name)
	if err != nil {
		uploadResult = "error"

		h.log.Errorw("failed to get npm package", "name", pub.Name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	for version, manifestRaw := range pub.Versions {
		if err := pkgnpm.ValidateSemver(version); err != nil {
			uploadResult = "error"

			http.Error(w, "invalid semver: "+version, http.StatusBadRequest)

			return
		}

		tarballFilename := formatnpm.TarballFilename(pub.Name, version)

		att, ok := pub.Attachments[tarballFilename]
		if !ok {
			uploadResult = "error"

			http.Error(w, "missing attachment: "+tarballFilename, http.StatusBadRequest)

			return
		}

		data, _, err := pkgnpm.DecodeAttachment(att)
		if err != nil {
			uploadResult = "error"

			http.Error(w, "invalid attachment: "+err.Error(), http.StatusBadRequest)

			return
		}

		integrity, sha1Hex, sha256Hex, _, err := pkgnpm.ComputeIntegrity(bytes.NewReader(data))
		if err != nil {
			uploadResult = "error"

			h.log.Errorw("failed to compute integrity", "pkg", pub.Name, "version", version, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		var manifestMap map[string]any

		dec2 := json.NewDecoder(bytes.NewReader(manifestRaw))
		dec2.UseNumber()

		if err := dec2.Decode(&manifestMap); err != nil {
			uploadResult = "error"

			http.Error(w, "invalid manifest JSON", http.StatusBadRequest)

			return
		}

		if dist, ok := manifestMap["dist"].(map[string]any); ok {
			if declaredIntegrity, ok := dist["integrity"].(string); ok && declaredIntegrity != "" {
				if declaredIntegrity != integrity {
					uploadResult = "error"

					http.Error(w, "integrity mismatch", http.StatusBadRequest)

					return
				}
			}
		}

		if pkg == nil {
			scope := ""

			name := pub.Name
			if strings.HasPrefix(pub.Name, "@") {
				if idx := strings.Index(pub.Name, "/"); idx > 0 {
					scope = pub.Name[:idx]
					name = pub.Name[idx+1:]
				}
			}

			pkg, err = h.db.CreateNPMPackage(r.Context(), &models.NPMPackage{
				RepositoryID:   repo.ID,
				Scope:          scope,
				Name:           name,
				FullName:       pub.Name,
				LatestVersion:  version,
				PackumentStale: true,
			})
			if err != nil {
				uploadResult = "error"

				h.log.Errorw("failed to create npm package", "name", pub.Name, "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)

				return
			}
		}

		_, err = h.blobStore.Put(r.Context(), bytes.NewReader(data), map[string]string{
			"sha256": sha256Hex,
			"format": "npm",
		})
		if err != nil {
			uploadResult = "error"

			h.log.Errorw("failed to store tarball blob", "pkg", pub.Name, "version", version, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		// Upsert into blobs table to get UUID for npm_versions.blob_id FK.
		blobUUID, err := h.db.UpsertBlob(r.Context(), sha256Hex, sha256Hex, int64(len(data)))
		if err != nil {
			uploadResult = "error"
			h.log.Errorw("failed to upsert npm blob", "pkg", pub.Name, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		h.log.Debugw("npm publish", "repo", repoName, "pkg", pub.Name, "version", version)

		_, err = h.db.CreateNPMVersion(r.Context(), &models.NPMVersion{
			PackageID:       pkg.ID,
			Version:         version,
			ManifestJSON:    manifestRaw,
			Integrity:       integrity,
			Shasum:          sha1Hex,
			BlobID:          blobUUID,
			TarballFilename: tarballFilename,
		})
		if err != nil {
			uploadResult = "error"

			if isUniqueViolation(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)

				if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "version already exists"}); encErr != nil {
					_ = encErr // response header already written
				}

				return
			}

			h.log.Errorw("failed to create npm version", "pkg", pub.Name, "version", version, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}
	}

	for tag, ver := range pub.DistTags {
		if _, err := h.db.UpsertNPMDistTag(r.Context(), &models.NPMDistTag{
			PackageID: pkg.ID,
			Tag:       tag,
			Version:   ver,
		}); err != nil {
			h.log.Errorw("failed to upsert dist-tag", "pkg", pub.Name, "tag", tag, "error", err)
		}
	}

	if err := h.db.MarkPackumentStale(r.Context(), pkg.ID); err != nil {
		h.log.Warnw("failed to mark packument stale", "pkg", pub.Name, "error", err)
	}

	if h.npmWorker != nil {
		h.npmWorker.NotifyStale(pkg.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]any{
		"ok":  true,
		"id":  pub.Name,
		"rev": "1-" + time.Now().UTC().Format("20060102150405"),
	}); encErr != nil {
		_ = encErr // response header already written
	}
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *NPMUploadHandler) handleUnpublish(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	vars := urlparams.Vars(r)
	repoName := vars["repo"]
	rawPkg := vars["*"]

	repo, err := h.resolveRepo(r.Context(), repoName)
	if err != nil {
		h.log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repo == nil {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(rawPkg, "/-rev/")
	pkgName := npmDecodePackageName(parts[0])

	var targetVersion string

	if len(parts) == 2 {
		revAndVersion := parts[1]
		if idx := strings.Index(revAndVersion, "/"); idx > 0 {
			targetVersion = revAndVersion[idx+1:]
		}
	}

	pkg, err := h.db.GetNPMPackageByName(r.Context(), repo.ID, pkgName)
	if err != nil {
		h.log.Errorw("failed to get npm package", "name", pkgName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if pkg == nil {
		http.NotFound(w, r)
		return
	}

	versions, err := h.db.GetNPMVersionsByPackage(r.Context(), pkg.ID)
	if err != nil {
		h.log.Errorw("failed to get npm versions", "pkg", pkgName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if targetVersion != "" { //nolint:nestif // complex version resolution logic
		var found *models.NPMVersion

		for _, v := range versions {
			if v.Version == targetVersion {
				found = v
				break
			}
		}

		if found == nil {
			http.NotFound(w, r)
			return
		}

		if time.Since(found.CreatedAt) > 72*time.Hour || repo.AllowFullUnpublish {
			if err := h.db.DeleteNPMVersion(r.Context(), pkg.ID, targetVersion); err != nil {
				h.log.Errorw("failed to delete npm version", "pkg", pkgName, "version", targetVersion, "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)

				return
			}

			if delErr := storage.SafeDeleteBlob(r.Context(), h.db, h.blobStore, found.BlobID); delErr != nil {
				h.log.Warnw("failed to clean up blob after unpublish", "sha256", found.BlobID, "error", delErr)
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)

			if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "cannot unpublish within 72h, use npm deprecate instead"}); encErr != nil {
				_ = encErr // response header already written
			}

			return
		}
	} else {
		for _, v := range versions {
			if time.Since(v.CreatedAt) <= 72*time.Hour && !repo.AllowFullUnpublish {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)

				if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "cannot unpublish within 72h, use npm deprecate instead"}); encErr != nil {
					_ = encErr // response header already written
				}

				return
			}
		}

		for _, v := range versions {
			if err := h.db.DeleteNPMVersion(r.Context(), pkg.ID, v.Version); err != nil {
				h.log.Errorw("failed to delete npm version", "pkg", pkgName, "version", v.Version, "error", err)
			}

			if delErr := storage.SafeDeleteBlob(r.Context(), h.db, h.blobStore, v.BlobID); delErr != nil {
				h.log.Warnw("failed to clean up blob after unpublish", "sha256", v.BlobID, "error", delErr)
			}
		}

		if err := h.db.DeleteNPMPackage(r.Context(), pkg.ID); err != nil {
			h.log.Errorw("failed to delete npm package", "pkg", pkgName, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}
	}

	if h.npmWorker != nil {
		h.npmWorker.NotifyStale(pkg.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *NPMUploadHandler) handlePutDistTag(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	vars := urlparams.Vars(r)
	repoName := vars["repo"]
	pkgName := npmDecodePackageName(vars["pkg"])
	tag := vars["tag"]

	repo, err := h.resolveRepo(r.Context(), repoName)
	if err != nil {
		h.log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repo == nil {
		http.NotFound(w, r)
		return
	}

	pkg, err := h.db.GetNPMPackageByName(r.Context(), repo.ID, pkgName)
	if err != nil {
		h.log.Errorw("failed to get npm package", "name", pkgName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if pkg == nil {
		http.NotFound(w, r)
		return
	}

	var version string
	if err := json.NewDecoder(r.Body).Decode(&version); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	v, err := h.db.GetNPMVersion(r.Context(), pkg.ID, version)
	if err != nil {
		h.log.Errorw("failed to get npm version", "pkg", pkgName, "version", version, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if v == nil {
		http.Error(w, "version not found", http.StatusNotFound)
		return
	}

	if _, err := h.db.UpsertNPMDistTag(r.Context(), &models.NPMDistTag{
		PackageID: pkg.ID,
		Tag:       tag,
		Version:   version,
	}); err != nil {
		h.log.Errorw("failed to upsert dist-tag", "pkg", pkgName, "tag", tag, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	distTags, err := h.db.GetNPMDistTags(r.Context(), pkg.ID)
	if err != nil {
		h.log.Errorw("failed to get npm dist-tags", "pkg", pkgName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if encErr := json.NewEncoder(w).Encode(distTags); encErr != nil {
		_ = encErr // response header already written
	}
}

func (h *NPMUploadHandler) handleDeleteDistTag(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	vars := urlparams.Vars(r)
	repoName := vars["repo"]
	pkgName := npmDecodePackageName(vars["pkg"])
	tag := vars["tag"]

	if tag == "latest" {
		http.Error(w, "cannot delete 'latest' dist-tag", http.StatusBadRequest)
		return
	}

	repo, err := h.resolveRepo(r.Context(), repoName)
	if err != nil {
		h.log.Errorw("failed to resolve repo", "name", repoName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if repo == nil {
		http.NotFound(w, r)
		return
	}

	pkg, err := h.db.GetNPMPackageByName(r.Context(), repo.ID, pkgName)
	if err != nil {
		h.log.Errorw("failed to get npm package", "name", pkgName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	if pkg == nil {
		http.NotFound(w, r)
		return
	}

	if err := h.db.DeleteNPMDistTag(r.Context(), pkg.ID, tag); err != nil {
		h.log.Errorw("failed to delete dist-tag", "pkg", pkgName, "tag", tag, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	distTags, err := h.db.GetNPMDistTags(r.Context(), pkg.ID)
	if err != nil {
		h.log.Errorw("failed to get npm dist-tags", "pkg", pkgName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if encErr := json.NewEncoder(w).Encode(distTags); encErr != nil {
		_ = encErr // response header already written
	}
}

func npmDecodePackageName(raw string) string {
	s := strings.ReplaceAll(raw, "%2F", "/")
	s = strings.ReplaceAll(s, "%2f", "/")

	return s
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "unique") ||
		strings.Contains(msg, "23505")
}
