// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/quota"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/webhook"
	"github.com/openartifact/openartifact/pkg/apk"
)

// apiPath is the management API path of a repository.
func apiPath(repoName string) string { return "/api/v1/repositories/" + repoName + "/apks" }

// writeInputError answers err as 400 when it is an input error, otherwise 500.
func (h *Handler) writeInputError(w http.ResponseWriter, err error, what string) {
	if msg, ok := isInputError(err); ok {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", msg)

		return
	}

	h.fail(w, what, err)
}

// pageOf is a page of results.
type pageOf struct {
	Items any `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// reverseOrder reports whether order (asc or desc) is the opposite of the
// default direction of sort: names ascend, everything else descends.
func reverseOrder(sort, order string) bool {
	defaultDesc := sort != "name"

	switch order {
	case "asc":
		return defaultDesc
	case "desc":
		return !defaultDesc
	}

	return false
}

type appItem struct {
	models.AndroidAppSummary

	IconURL string `json:"icon_url,omitempty"`
}

func (h *Handler) handleListApps(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	q := r.URL.Query()
	page, limit, offset := pageParams(r, 50)

	f := models.AndroidAppFilter{
		Query: strings.TrimSpace(q.Get("q")), Origin: q.Get("origin"), Sort: q.Get("sort"),
		Reverse: reverseOrder(q.Get("sort"), q.Get("order")),
		Limit:   limit, Offset: offset,
	}

	tags, err := normalizeTags(queryList(r, "tag"))
	if err != nil {
		h.writeInputError(w, err, "invalid tag filter")

		return
	}

	f.Tags = tags

	if f.Origin != "" && !validOrigin(f.Origin) {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "origin must be one of built, patched, original, unknown")

		return
	}

	if s := q.Get("signer"); s != "" {
		sig, ok := apk.NormalizeFingerprint(s)
		if !ok {
			writeError(w, http.StatusBadRequest, "INVALID_INPUT", "signer must be a SHA-256 fingerprint")

			return
		}

		f.Signer = sig
	}

	apps, total, err := h.store.ListAndroidApps(r.Context(), repo.ID, f)
	if err != nil {
		h.fail(w, "failed to list apps", err)

		return
	}

	items := make([]appItem, len(apps))
	for i := range apps {
		items[i] = appItem{AndroidAppSummary: apps[i]}
		if apps[i].LatestID != "" {
			items[i].IconURL = apiPath(repo.Name) + "/versions/" + apps[i].LatestID + "/icon?size=96"
		}
	}

	writeOK(w, http.StatusOK, pageOf{Items: items, Total: total, Page: page, Limit: limit})
}

// signerView is a key used by an app together with how the repository knows it.
type signerView struct {
	models.AndroidSignerUse

	Known bool   `json:"known"`
	Trust string `json:"trust,omitempty"`
	Label string `json:"label,omitempty"`
}

func (h *Handler) appByPackage(w http.ResponseWriter, r *http.Request, repo *models.Repository) *models.AndroidApp {
	app, err := h.store.GetAndroidApp(r.Context(), repo.ID, urlparams.Vars(r)["pkg"])
	if err != nil {
		h.fail(w, "failed to look up the app", err)

		return nil
	}

	if app == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "app not found")
	}

	return app
}

func (h *Handler) handleGetApp(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	app := h.appByPackage(w, r, repo)
	if app == nil {
		return
	}

	use, err := h.store.ListAndroidAppSignerUse(r.Context(), repo.ID, app.ID)
	if err != nil {
		h.fail(w, "failed to list signers", err)

		return
	}

	known, err := h.store.ListAndroidSigners(r.Context(), repo.ID, app.ID)
	if err != nil {
		h.fail(w, "failed to list signers", err)

		return
	}

	signers := make([]signerView, len(use))

	for i, u := range use {
		signers[i] = signerView{AndroidSignerUse: u}

		// An app-scoped registration wins over a repository-wide one.
		for _, k := range known {
			if k.SHA256 == u.SHA256 && (!signers[i].Known || k.AppID != "") {
				signers[i].Known, signers[i].Trust, signers[i].Label = true, k.Trust, k.Label
			}
		}
	}

	writeOK(w, http.StatusOK, map[string]any{"app": app, "signers": signers})
}

// appMetaBody is the body of PUT apps/{pkg}/meta; absent fields stay unchanged.
type appMetaBody struct {
	LabelOverride  *string   `json:"label_override"`
	Notes          *string   `json:"notes"`
	Tags           *[]string `json:"tags"`
	SignerPolicy   *string   `json:"signer_policy"`
	UpstreamKind   *string   `json:"upstream_kind"`
	UpstreamRef    *string   `json:"upstream_ref"`
	UpstreamLatest *string   `json:"upstream_latest"`
}

func (b *appMetaBody) toMeta() (models.AndroidAppMeta, error) {
	m := models.AndroidAppMeta{
		LabelOverride: b.LabelOverride, Notes: b.Notes, SignerPolicy: b.SignerPolicy,
		UpstreamKind: b.UpstreamKind, UpstreamRef: b.UpstreamRef, UpstreamLatest: b.UpstreamLatest,
	}

	if b.Notes != nil {
		if err := checkText("notes", *b.Notes); err != nil {
			return m, err
		}
	}

	if b.LabelOverride != nil && len(*b.LabelOverride) > 255 {
		return m, inputError("label_override is longer than 255 bytes")
	}

	if b.SignerPolicy != nil {
		switch *b.SignerPolicy {
		case models.AndroidPolicyAllow, models.AndroidPolicyWarn, models.AndroidPolicyReject:
		default:
			return m, inputError("signer_policy must be allow, warn or reject")
		}
	}

	if b.Tags != nil {
		tags, err := normalizeTags(*b.Tags)
		if err != nil {
			return m, err
		}

		m.Tags = tags
	}

	return m, nil
}

func (h *Handler) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	app := h.appByPackage(w, r, repo)
	if app == nil {
		return
	}

	var body appMetaBody
	if err := decodeJSON(w, r, &body); err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	meta, err := body.toMeta()
	if err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	updated, err := h.store.UpdateAndroidAppMeta(r.Context(), repo.ID, app.ID, meta)
	if err != nil {
		h.fail(w, "failed to update the app", err)

		return
	}

	if updated == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "app not found")

		return
	}

	writeOK(w, http.StatusOK, updated)
}

// versionView is a version with the URLs a client needs.
type versionView struct {
	models.AndroidVersion

	DownloadURL string `json:"download_url"`
	IconURL     string `json:"icon_url"`
}

func (h *Handler) view(repoName string, v *models.AndroidVersion) versionView {
	return versionView{
		AndroidVersion: *v,
		DownloadURL:    h.BaseURL + downloadPath(repoName, v),
		IconURL:        apiPath(repoName) + "/versions/" + v.ID + "/icon?size=96",
	}
}

func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	app := h.appByPackage(w, r, repo)
	if app == nil {
		return
	}

	q := r.URL.Query()
	page, limit, offset := pageParams(r, 100)

	f := models.AndroidVersionFilter{AppID: app.ID, Origin: q.Get("origin"), Limit: limit, Offset: offset}

	tags, err := normalizeTags(queryList(r, "tag"))
	if err != nil {
		h.writeInputError(w, err, "invalid tag filter")

		return
	}

	f.Tags = tags

	if s := q.Get("signer"); s != "" {
		sig, ok := apk.NormalizeFingerprint(s)
		if !ok {
			writeError(w, http.StatusBadRequest, "INVALID_INPUT", "signer must be a SHA-256 fingerprint")

			return
		}

		f.Signer = sig
	}

	vs, total, err := h.store.ListAndroidVersions(r.Context(), repo.ID, f)
	if err != nil {
		h.fail(w, "failed to list versions", err)

		return
	}

	items := make([]versionView, len(vs))
	for i := range vs {
		items[i] = h.view(repo.Name, &vs[i])
	}

	writeOK(w, http.StatusOK, pageOf{Items: items, Total: total, Page: page, Limit: limit})
}

// versionOf loads the version named in the URL, answering 404 itself.
func (h *Handler) versionOf(w http.ResponseWriter, r *http.Request, repo *models.Repository, detail bool) *models.AndroidVersion {
	v, err := h.store.GetAndroidVersion(r.Context(), repo.ID, urlparams.Vars(r)["id"], detail)
	if err != nil {
		h.fail(w, "failed to look up the version", err)

		return nil
	}

	if v == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "version not found")
	}

	return v
}

// parentView is the summary of the original a version derives from.
type parentView struct {
	ID           string `json:"id"`
	VersionName  string `json:"version_name"`
	VersionCode  int64  `json:"version_code"`
	Origin       string `json:"origin"`
	SignerSHA256 string `json:"signer_sha256"`
	// SameSigner and SameContent tell a re-signed copy from a modified one.
	SameSigner  bool `json:"same_signer"`
	SameContent bool `json:"same_content"`
}

func (h *Handler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	v := h.versionOf(w, r, repo, true)
	if v == nil {
		return
	}

	ctx := r.Context()
	out := map[string]any{"version": h.view(repo.Name, v), "signer": nil, "parent": nil}

	if v.SignerSHA256 != "" {
		sv, err := h.signerOf(ctx, repo.ID, v)
		if err != nil {
			h.fail(w, "failed to look up the signer", err)

			return
		}

		out["signer"] = sv
	}

	if v.ParentID != "" {
		p, err := h.store.GetAndroidVersion(ctx, repo.ID, v.ParentID, false)
		if err != nil {
			h.fail(w, "failed to look up the parent", err)

			return
		}

		if p != nil {
			out["parent"] = parentView{
				ID: p.ID, VersionName: p.VersionName, VersionCode: p.VersionCode, Origin: p.Origin, SignerSHA256: p.SignerSHA256,
				SameSigner: p.SignerSHA256 == v.SignerSHA256, SameContent: p.ContentDigest != "" && p.ContentDigest == v.ContentDigest,
			}
		}
	}

	icons, err := h.store.ListAndroidIcons(ctx, repo.ID, v.ID)
	if err != nil {
		h.fail(w, "failed to list icons", err)

		return
	}

	files, err := h.store.ListAndroidFiles(ctx, repo.ID, v.ID)
	if err != nil {
		h.fail(w, "failed to list files", err)

		return
	}

	out["icons"], out["files"] = h.iconViews(repo.Name, v.ID, icons), files

	writeOK(w, http.StatusOK, out)
}

// signerOf describes how the repository knows the signer of v.
func (h *Handler) signerOf(ctx context.Context, repoID string, v *models.AndroidVersion) (signerView, error) {
	sv := signerView{SHA256: v.SignerSHA256}

	known, err := h.store.ListAndroidSigners(ctx, repoID, v.AppID)
	if err != nil {
		return sv, err //nolint:wrapcheck // wrapped by the caller's message
	}

	for _, k := range known {
		if k.SHA256 == v.SignerSHA256 && (!sv.Known || k.AppID != "") {
			sv.Known, sv.Trust, sv.Label = true, k.Trust, k.Label
		}
	}

	return sv, nil
}

// versionMetaBody is the body of PUT versions/{id}/meta; absent fields stay unchanged.
type versionMetaBody struct {
	Notes     *string         `json:"notes"`
	Changelog *string         `json:"changelog"`
	Tags      *[]string       `json:"tags"`
	Channel   *string         `json:"channel"`
	Origin    *string         `json:"origin"`
	ParentID  *string         `json:"parent_id"`
	SourceURL *string         `json:"source_url"`
	PatchInfo json.RawMessage `json:"patch_info"`
	Pinned    *bool           `json:"pinned"`
}

func (b *versionMetaBody) toMeta() (models.AndroidVersionMeta, error) {
	m := models.AndroidVersionMeta{
		Notes: b.Notes, Changelog: b.Changelog, Origin: b.Origin, ParentID: b.ParentID, SourceURL: b.SourceURL,
		PatchInfo: b.PatchInfo, Pinned: b.Pinned,
	}

	for name, s := range map[string]*string{"notes": b.Notes, "changelog": b.Changelog} {
		if s != nil {
			if err := checkText(name, *s); err != nil {
				return m, err
			}
		}
	}

	if b.Origin != nil && !validOrigin(*b.Origin) {
		return m, inputError("origin must be one of built, patched, original, unknown")
	}

	if b.ParentID != nil && *b.ParentID != "" && uuid.Validate(*b.ParentID) != nil {
		return m, inputError("parent_id must be a version id or empty")
	}

	if b.SourceURL != nil {
		if err := checkSourceURL(*b.SourceURL); err != nil {
			return m, err
		}
	}

	if err := checkPatchInfo(b.PatchInfo); err != nil {
		return m, err
	}

	if b.Channel != nil {
		ch, err := normalizeChannel(*b.Channel)
		if err != nil {
			return m, err
		}

		m.Channel = &ch
	}

	if b.Tags != nil {
		tags, err := normalizeTags(*b.Tags)
		if err != nil {
			return m, err
		}

		m.Tags = tags
	}

	return m, nil
}

func (h *Handler) handleUpdateVersion(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	var body versionMetaBody
	if err := decodeJSON(w, r, &body); err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	meta, err := body.toMeta()
	if err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	v, err := h.store.UpdateAndroidVersionMeta(r.Context(), repo.ID, urlparams.Vars(r)["id"], meta)
	if errors.Is(err, database.ErrAndroidBadParent) {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "parent_id is not a version of this repository")

		return
	}

	if err != nil {
		h.fail(w, "failed to update the version", err)

		return
	}

	if v == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "version not found")

		return
	}

	writeOK(w, http.StatusOK, h.view(repo.Name, v))
}

func (h *Handler) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	id := urlparams.Vars(r)["id"]

	del, err := h.store.DeleteAndroidVersion(r.Context(), repo.ID, id)
	if err != nil {
		h.fail(w, "failed to delete the version", err)

		return
	}

	if del == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "version not found")

		return
	}

	h.releaseDeleted(r.Context(), repo, del)

	v := del.Version
	h.Emit(r.Context(), repo.ID, webhook.EventArtifactDeleted, webhook.ArtifactPayload(webhook.EventArtifactDeleted, webhook.Artifact{
		Repository: repo.Name, Format: Format, Name: v.PackageName, Version: v.VersionName, Path: v.Filename, SHA256: v.SHA256, Size: v.SizeBytes,
	}))

	h.Log.Infow("Android version deleted", "repository", repo.Name, "version_id", id)
	writeOK(w, http.StatusOK, map[string]any{"repository": repo.Name, "version_id": id})
}

// releaseDeleted gives back quota and storage of a deleted version.
func (h *Handler) releaseDeleted(ctx context.Context, repo *models.Repository, del *database.AndroidDeleted) {
	if err := quota.ReleaseN(ctx, h.DB, repo.ID, del.QuotaBytes, del.QuotaCount); err != nil {
		h.Log.Warnw("failed to release quota", "repository", repo.Name, "error", err)
	}

	for _, sha := range del.BlobSHA256 {
		if err := storage.SafeDeleteBlob(ctx, h.DB, h.writeBlob, sha); err != nil {
			h.Log.Warnw("failed to delete blob", "sha256", sha, "error", err)
		}
	}
}

func (h *Handler) handleManifestXML(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	v := h.versionOf(w, r, repo, true)
	if v == nil {
		return
	}

	// Plain text, never rendered as XML or HTML by the browser.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-cache")
	_, _ = w.Write([]byte(v.ManifestXML))
}
