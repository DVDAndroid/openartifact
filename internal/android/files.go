// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/quota"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
)

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// iconView is an icon with the URL its image is served from.
type iconView struct {
	models.AndroidIcon

	URL string `json:"url,omitempty"`
}

func (h *Handler) iconViews(repoName, versionID string, icons []models.AndroidIcon) []iconView {
	out := make([]iconView, len(icons))

	for i, ic := range icons {
		out[i] = iconView{AndroidIcon: ic}
		if ic.BlobID != "" {
			out[i].URL = apiPath(repoName) + "/versions/" + versionID + "/icons/" + ic.ID
		}
	}

	return out
}

func (h *Handler) handleListIcons(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	v := h.versionOf(w, r, repo, false)
	if v == nil {
		return
	}

	icons, err := h.store.ListAndroidIcons(r.Context(), repo.ID, v.ID)
	if err != nil {
		h.fail(w, "failed to list icons", err)

		return
	}

	writeOK(w, http.StatusOK, map[string]any{
		"icons": h.iconViews(repo.Name, v.ID, icons), "label": v.Label, "launcher_label": v.LauncherLabel, "labels": v.Labels,
	})
}

// imageContentType returns the type an icon is served as: raster images as
// themselves, anything else (decoded vector XML, colors) as inert text.
func imageContentType(mime string) string {
	switch mime {
	case "image/png", "image/webp", "image/jpeg", "image/gif":
		return mime
	}

	return "text/plain; charset=utf-8"
}

// pickIcon chooses the icon of role closest to size pixels: the smallest one
// at least that large, else the largest. Only raster images with stored data
// qualify. size <= 0 asks for the largest.
func pickIcon(icons []models.AndroidIcon, role string, size int) *models.AndroidIcon {
	var best *models.AndroidIcon

	for i := range icons {
		ic := &icons[i]
		if ic.Role != role || ic.BlobID == "" || imageContentType(ic.Mime) != ic.Mime {
			continue
		}

		if best == nil || betterIcon(ic, best, size) {
			best = ic
		}
	}

	return best
}

func betterIcon(a, b *models.AndroidIcon, size int) bool {
	aFits, bFits := size > 0 && a.Width >= size, size > 0 && b.Width >= size

	switch {
	case aFits && bFits:
		return a.Width < b.Width
	case aFits != bFits:
		return aFits
	}

	return a.Width > b.Width
}

// streamStored writes a stored blob as a response with inert headers.
func (h *Handler) streamStored(w http.ResponseWriter, r *http.Request, repo *models.Repository, sha, contentType, disposition string, size int64) {
	hdr := w.Header()
	hdr.Set("Content-Type", contentType)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "default-src 'none'; sandbox")

	if disposition != "" {
		hdr.Set("Content-Disposition", disposition)
	}

	if base.Revalidate(w, r, repo.Public, `"`+sha+`"`, metadataMaxAge) {
		return
	}

	rc, err := h.BlobStore.Get(r.Context(), sha)
	if err != nil {
		h.Log.Errorw("failed to read android blob", "sha256", sha, "error", err)
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to read the file")

		return
	}
	defer base.CloseLog(rc, h.Log)

	hdr.Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		h.Log.Warnw("android response interrupted", "sha256", sha, "error", err)
	}
}

func (h *Handler) handleGetIcon(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	vars := urlparams.Vars(r)

	ic, err := h.store.GetAndroidIcon(r.Context(), repo.ID, vars["id"], vars["iconId"])
	if err != nil {
		h.fail(w, "failed to look up the icon", err)

		return
	}

	if ic == nil || ic.BlobID == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "icon not found")

		return
	}

	h.streamStored(w, r, repo, ic.SHA256, imageContentType(ic.Mime), "", ic.SizeBytes)
}

func (h *Handler) handleBestIcon(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	v := h.versionOf(w, r, repo, false)
	if v == nil {
		return
	}

	icons, err := h.store.ListAndroidIcons(r.Context(), repo.ID, v.ID)
	if err != nil {
		h.fail(w, "failed to list icons", err)

		return
	}

	role := r.URL.Query().Get("role")
	if role == "" {
		role = "app"
	}

	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	ic := pickIcon(icons, role, size)
	if ic == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "no icon")

		return
	}

	h.streamStored(w, r, repo, ic.SHA256, imageContentType(ic.Mime), "", ic.SizeBytes)
}

func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	v := h.versionOf(w, r, repo, false)
	if v == nil {
		return
	}

	files, err := h.store.ListAndroidFiles(r.Context(), repo.ID, v.ID)
	if err != nil {
		h.fail(w, "failed to list files", err)

		return
	}

	if files == nil {
		files = []models.AndroidFile{}
	}

	writeOK(w, http.StatusOK, files)
}

func validFileKind(k string) bool {
	switch k {
	case "mapping", "symbols", "patch_bundle", "patch_options", "source", "screenshot", "original_apk", "other":
		return true
	}

	return false
}

// safeFileName reduces an uploaded name to a harmless base name.
func safeFileName(name string) string {
	name = unsafeNameChars.ReplaceAllString(path.Base(strings.ReplaceAll(name, `\`, "/")), "_")
	name = strings.TrimLeft(name, ".")

	if len(name) > 200 {
		name = name[:200]
	}

	if name == "" {
		return "file"
	}

	return name
}

func (h *Handler) handleAttachFile(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	v := h.versionOf(w, r, repo, false)
	if v == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload+multipartOverhead)

	if err := r.ParseMultipartForm(multipartMemory); err != nil { //nolint:gosec // r.Body is bounded by http.MaxBytesReader above
		h.writeUploadError(w, inputErrorf("failed to parse form: %v", err), "attach")

		return
	}

	kind := r.FormValue("kind")
	if kind == "" {
		kind = "other"
	}

	headers := r.MultipartForm.File["file"]
	if !validFileKind(kind) || len(headers) != 1 {
		h.writeUploadError(w, inputError("send one file and a kind (mapping, symbols, patch_bundle, patch_options, source, screenshot, original_apk, other)"), "attach")

		return
	}

	f, err := h.storeFile(r.Context(), repo, v, kind, r.FormValue("name"), headers[0])
	if err != nil {
		h.writeUploadError(w, err, "attach")

		return
	}

	writeOK(w, http.StatusCreated, f)
}

// storeFile buffers an attachment, stores its blob and links it to v.
func (h *Handler) storeFile(ctx context.Context, repo *models.Repository, v *models.AndroidVersion, kind, name string, fh *multipart.FileHeader) (*models.AndroidFile, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open upload: %w", err)
	}
	defer base.CloseLog(src, h.Log)

	sp, err := spool(src, h.maxUpload)
	if err != nil {
		return nil, err
	}
	defer sp.close()

	if name == "" {
		name = fh.Filename
	}

	f := &models.AndroidFile{VersionID: v.ID, Kind: kind, Name: safeFileName(name), SHA256: sp.sha, SizeBytes: sp.size}

	if err := quota.Enforce(ctx, h.DB, repo.ID, sp.size); err != nil {
		return nil, err //nolint:wrapcheck // callers compare with quota.ErrExceeded
	}

	puts := &blobPut{h: h, repo: repo.Name}

	err = storage.WithBlobLock(ctx, h.DB, sp.sha, func(ctx context.Context) error {
		if _, err := sp.f.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to rewind upload: %w", err)
		}

		if f.BlobID, err = puts.put(ctx, sp.sha, sp.size, sp.f); err != nil {
			return err
		}

		return h.store.CreateAndroidFile(ctx, repo.ID, f)
	})
	if err != nil {
		if rerr := quota.Release(ctx, h.DB, repo.ID, sp.size); rerr != nil {
			h.Log.Warnw("failed to release quota", "repository", repo.Name, "error", rerr)
		}

		h.dropBlobs(ctx, puts.created)

		return nil, err
	}

	return f, nil
}

func (h *Handler) handleGetFile(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	vars := urlparams.Vars(r)

	f, err := h.store.GetAndroidFile(r.Context(), repo.ID, vars["id"], vars["fid"])
	if err != nil {
		h.fail(w, "failed to look up the file", err)

		return
	}

	if f == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "file not found")

		return
	}

	// Attachments are user supplied: always a download, never rendered.
	h.streamStored(w, r, repo, f.SHA256, "application/octet-stream", `attachment; filename="`+f.Name+`"`, f.SizeBytes)
}

func (h *Handler) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	vars := urlparams.Vars(r)

	f, err := h.store.DeleteAndroidFile(r.Context(), repo.ID, vars["id"], vars["fid"])
	if err != nil {
		h.fail(w, "failed to delete the file", err)

		return
	}

	if f == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "file not found")

		return
	}

	if err := quota.Release(r.Context(), h.DB, repo.ID, f.SizeBytes); err != nil {
		h.Log.Warnw("failed to release quota", "repository", repo.Name, "error", err)
	}

	if err := storage.SafeDeleteBlob(r.Context(), h.DB, h.writeBlob, f.SHA256); err != nil {
		h.Log.Warnw("failed to delete blob", "sha256", f.SHA256, "error", err)
	}

	writeOK(w, http.StatusOK, map[string]any{"file_id": f.ID})
}
