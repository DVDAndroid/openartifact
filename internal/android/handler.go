// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package android serves the "android" repository format: an archive of APK
// files that records who signed each one, where it came from and what was
// patched. APK parsing lives in pkg/apk; this package is the HTTP side.
//
// Two route groups are mounted by cmd/server:
//
//   - RegisterRepoRoutes: read-only routes for clients under /repos/{name}
//     (download, latest.apk, latest.json, index.json);
//   - Register: the management API under /api/v1/repositories/{name}/apks
//     (upload, inspect, apps, versions, tags, notes, known keys).
//
// Configuration is read from the environment here, not from internal/config:
// OPENARTIFACT_ANDROID_MAX_UPLOAD_MB (default 2048).
package android

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/repotype"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

// Format is the repository format served by this package.
const Format = "android"

const (
	defaultMaxUploadMB = 2048
	// multipartOverhead leaves room for form fields and boundaries on top of the files.
	multipartOverhead = 1 << 20
	// multipartMemory is the in-memory threshold; larger parts spill to disk.
	multipartMemory = 32 << 20
)

// Store is what the handlers need from the database beyond database.Database.
// *database.PostgresDB implements it.
type Store interface {
	UpsertAndroidApp(ctx context.Context, repoID, packageName string) (*models.AndroidApp, error)
	GetAndroidApp(ctx context.Context, repoID, packageName string) (*models.AndroidApp, error)
	GetAndroidAppByID(ctx context.Context, repoID, id string) (*models.AndroidApp, error)
	ListAndroidApps(ctx context.Context, repoID string, f models.AndroidAppFilter) ([]models.AndroidAppSummary, int, error)
	UpdateAndroidAppMeta(ctx context.Context, repoID, id string, m models.AndroidAppMeta) (*models.AndroidApp, error)

	CreateAndroidVersion(ctx context.Context, v *models.AndroidVersion, icons []models.AndroidIcon) error
	GetAndroidVersion(ctx context.Context, repoID, id string, detail bool) (*models.AndroidVersion, error)
	GetAndroidVersionBySHA256(ctx context.Context, repoID, sha string) (*models.AndroidVersion, error)
	GetAndroidDownload(ctx context.Context, repoID, packageName string, versionCode int64, filename string) (*models.AndroidVersion, error)
	FindAndroidOriginal(ctx context.Context, repoID, packageName string, versionCode int64) (*models.AndroidVersion, error)
	ListAndroidVersions(ctx context.Context, repoID string, f models.AndroidVersionFilter) ([]models.AndroidVersion, int, error)
	LatestAndroidVersion(ctx context.Context, repoID, packageName string, f database.AndroidLatestFilter) (*models.AndroidVersion, error)
	UpdateAndroidVersionMeta(ctx context.Context, repoID, id string, m models.AndroidVersionMeta) (*models.AndroidVersion, error)
	DeleteAndroidVersion(ctx context.Context, repoID, id string) (*database.AndroidDeleted, error)
	IncrementAndroidDownload(ctx context.Context, id string) error

	ListAndroidIcons(ctx context.Context, repoID, versionID string) ([]models.AndroidIcon, error)
	GetAndroidIcon(ctx context.Context, repoID, versionID, iconID string) (*models.AndroidIcon, error)

	CreateAndroidFile(ctx context.Context, repoID string, f *models.AndroidFile) error
	ListAndroidFiles(ctx context.Context, repoID, versionID string) ([]models.AndroidFile, error)
	GetAndroidFile(ctx context.Context, repoID, versionID, fileID string) (*models.AndroidFile, error)
	DeleteAndroidFile(ctx context.Context, repoID, versionID, fileID string) (*models.AndroidFile, error)

	ListAndroidSigners(ctx context.Context, repoID, appID string) ([]models.AndroidSigner, error)
	UpsertAndroidSigner(ctx context.Context, s *models.AndroidSigner) error
	UpdateAndroidSigner(ctx context.Context, repoID, id, label, trust, notes string) (*models.AndroidSigner, error)
	DeleteAndroidSigner(ctx context.Context, repoID, id string) (bool, error)
	AndroidSignerTrust(ctx context.Context, repoID, appID, sha string) (trust string, found bool, err error)
	ListAndroidAppSignerUse(ctx context.Context, repoID, appID string) ([]models.AndroidSignerUse, error)
	ListAndroidTags(ctx context.Context, repoID string) ([]models.AndroidTagCount, error)
}

var _ Store = (*database.PostgresDB)(nil)

// Handler serves the android format.
type Handler struct {
	*base.Handler

	store     Store
	writeBlob storage.BlobStore
	reg       *metrics.Registry
	maxUpload int64
}

// NewHandler builds the handler. servedStore is the presign-aware store used
// for downloads, writeStore the plain one used to store uploads.
func NewHandler(log *logger.Logger, db database.Database, store Store, servedStore, writeStore storage.BlobStore, baseURL string, reg *metrics.Registry) *Handler {
	return &Handler{
		Handler:   base.New(log, db, servedStore, baseURL),
		store:     store,
		writeBlob: writeStore,
		reg:       reg,
		maxUpload: maxUploadBytes(),
	}
}

func maxUploadBytes() int64 {
	mb, err := strconv.ParseInt(os.Getenv("OPENARTIFACT_ANDROID_MAX_UPLOAD_MB"), 10, 64)
	if err != nil || mb <= 0 {
		mb = defaultMaxUploadMB
	}

	return mb << 20
}

// RegisterRepoRoutes registers the client routes on the /repos/{name} subrouter.
func (h *Handler) RegisterRepoRoutes(r chi.Router) {
	r.Get("/apps/{pkg}/latest.apk", h.handleLatestAPK)
	r.Head("/apps/{pkg}/latest.apk", h.handleLatestAPK)
	r.Get("/apps/{pkg}/latest.json", h.handleLatestJSON)
	r.Get("/apps/{pkg}/index.json", h.handleIndexJSON)
	r.Get("/apps/{pkg}/{code}/{filename}", h.handleDownload)
	r.Head("/apps/{pkg}/{code}/{filename}", h.handleDownload)
}

// Register registers the management API on router.
func (h *Handler) Register(router chi.Router) {
	router.Route("/api/v1/repositories/{name}/apks", func(r chi.Router) {
		r.Post("/", h.handleUpload)
		r.Post("/inspect", h.handleInspect)

		r.Get("/apps", h.handleListApps)
		r.Get("/apps/{pkg}", h.handleGetApp)
		r.Put("/apps/{pkg}/meta", h.handleUpdateApp)
		r.Get("/apps/{pkg}/versions", h.handleListVersions)

		r.Get("/versions/{id}", h.handleGetVersion)
		r.Put("/versions/{id}/meta", h.handleUpdateVersion)
		r.Delete("/versions/{id}", h.handleDeleteVersion)
		r.Get("/versions/{id}/manifest.xml", h.handleManifestXML)
		r.Get("/versions/{id}/icons", h.handleListIcons)
		r.Get("/versions/{id}/icons/{iconId}", h.handleGetIcon)
		r.Get("/versions/{id}/icon", h.handleBestIcon)
		r.Get("/versions/{id}/files", h.handleListFiles)
		r.Post("/versions/{id}/files", h.handleAttachFile)
		r.Get("/versions/{id}/files/{fid}", h.handleGetFile)
		r.Delete("/versions/{id}/files/{fid}", h.handleDeleteFile)

		r.Get("/signers", h.handleListSigners)
		r.Post("/signers", h.handleCreateSigner)
		r.Put("/signers/{id}", h.handleUpdateSigner)
		r.Delete("/signers/{id}", h.handleDeleteSigner)

		r.Get("/tags", h.handleListTags)
	})
	h.Log.Infow("Android handlers registered")
}

// repoOf loads the repository named in the URL. It answers 404 and returns nil
// for a missing repository or one of another format; with write set, proxy and
// group repositories are refused with 405 as read-only.
func (h *Handler) repoOf(w http.ResponseWriter, r *http.Request, write bool) *models.Repository {
	repo, err := h.ResolveRepo(r.Context(), urlparams.Vars(r)["name"])
	if err != nil {
		h.Log.Errorw("failed to look up repository", "error", err)
		writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to look up repository")

		return nil
	}

	if repo == nil || repo.Format != Format {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "android repository not found")

		return nil
	}

	if write && repo.RepoType != "" && repo.RepoType != repotype.Hosted {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_INPUT", "proxy and group repositories are read-only")

		return nil
	}

	return repo
}

// fail logs err and answers 500 with a generic message.
func (h *Handler) fail(w http.ResponseWriter, what string, err error) {
	h.Log.Errorw(what, "error", err)
	writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", what)
}
