// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/middleware"
	"github.com/openartifact/openartifact/internal/quota"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/internal/webhook"
	"github.com/openartifact/openartifact/pkg/apk"
)

const maxEntriesStored = 50_000

// conflictError is a refused upload (duplicate file, rejected signer).
type conflictError struct {
	message string
	details map[string]any
}

func (e *conflictError) Error() string { return e.message }

// uploadForm holds the text fields of an upload.
type uploadForm struct {
	origin          string
	channel         string
	changelog       string
	notes           string
	sourceURL       string
	tags            []string
	patchInfo       json.RawMessage
	parent          string
	acceptNewSigner bool
	noOriginal      bool
}

func formBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}

	return false
}

func parseUploadForm(r *http.Request) (*uploadForm, error) {
	f := &uploadForm{
		origin:          strings.TrimSpace(r.FormValue("origin")),
		changelog:       r.FormValue("changelog"),
		notes:           r.FormValue("notes"),
		sourceURL:       strings.TrimSpace(r.FormValue("source_url")),
		parent:          strings.TrimSpace(r.FormValue("parent")),
		acceptNewSigner: formBool(r.FormValue("accept_new_signer")),
		noOriginal:      formBool(r.FormValue("no_original")),
	}

	if f.origin == "" {
		f.origin = models.AndroidOriginUnknown
	}

	if !validOrigin(f.origin) {
		return nil, inputErrorf("origin must be one of built, patched, original, unknown")
	}

	var err error

	if f.channel, err = normalizeChannel(r.FormValue("channel")); err != nil {
		return nil, err
	}

	if f.tags, err = normalizeTags(splitTags(r.FormValue("tags"))); err != nil {
		return nil, err
	}

	if raw := strings.TrimSpace(r.FormValue("patch_info")); raw != "" {
		f.patchInfo = json.RawMessage(raw)
	}

	for name, text := range map[string]string{"changelog": f.changelog, "notes": f.notes} {
		if err := checkText(name, text); err != nil {
			return nil, err
		}
	}

	if err := checkSourceURL(f.sourceURL); err != nil {
		return nil, err
	}

	if err := checkPatchInfo(f.patchInfo); err != nil {
		return nil, err
	}

	return f, nil
}

// candidate is one APK of an upload, buffered and inspected.
type candidate struct {
	sp       *spooled
	info     *apk.Info
	filename string
}

func (c *candidate) close() {
	if c != nil {
		c.sp.close()
	}
}

func (c *candidate) signer() string { return c.info.SignerSHA256() }

// upload is a parsed multipart upload.
type upload struct {
	form *uploadForm
	main *candidate
	orig *candidate
}

func (u *upload) close() {
	u.main.close()
	u.orig.close()
}

// readCandidate buffers a multipart file and inspects it as an APK.
func (h *Handler) readCandidate(fh *multipart.FileHeader) (*candidate, error) {
	switch strings.ToLower(path.Ext(fh.Filename)) {
	case ".apks", ".xapk", ".apkm":
		return nil, inputError("bundles (.apks, .xapk, .apkm) are not supported yet; upload the base .apk")
	}

	src, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open upload: %w", err)
	}
	defer base.CloseLog(src, h.Log)

	sp, err := spool(src, h.maxUpload)
	if err != nil {
		return nil, err
	}

	info, err := apk.Inspect(sp.f, sp.size, apk.Options{})
	if err != nil {
		sp.close()

		if errors.Is(err, apk.ErrNotZip) || errors.Is(err, apk.ErrNoManifest) {
			return nil, inputErrorf("not a valid APK: %v", err)
		}

		return nil, fmt.Errorf("failed to inspect APK: %w", err)
	}

	if err := checkInfo(info); err != nil {
		sp.close()

		return nil, err
	}

	return &candidate{sp: sp, info: info, filename: downloadName(info.Manifest.Package, info.Manifest.VersionCode, info.SignerSHA256())}, nil
}

// checkInfo refuses what cannot be stored as one version.
func checkInfo(info *apk.Info) error {
	m := &info.Manifest

	switch {
	case !validPackageName(m.Package):
		return inputErrorf("the manifest package name %q is not a valid Android package name", m.Package)
	case m.VersionCode < 0:
		return inputError("the manifest versionCode is negative")
	case m.IsSplit:
		return inputError("this is a split APK; upload splits as one bundle (.apks, .xapk, .apkm) once bundles are supported")
	}

	return nil
}

// readUpload parses the multipart body of an upload.
func (h *Handler) readUpload(w http.ResponseWriter, r *http.Request) (*upload, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 2*h.maxUpload+multipartOverhead)

	if err := r.ParseMultipartForm(multipartMemory); err != nil { //nolint:gosec // r.Body is bounded by http.MaxBytesReader above
		if _, tooBig := errors.AsType[*http.MaxBytesError](err); tooBig {
			return nil, errTooLarge
		}

		return nil, inputErrorf("failed to parse form: %v", err)
	}

	form, err := parseUploadForm(r)
	if err != nil {
		return nil, err
	}

	up := &upload{form: form}

	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		return nil, inputError("send exactly one APK in the file field")
	}

	if up.main, err = h.readCandidate(files[0]); err != nil {
		return nil, err
	}

	if orig := r.MultipartForm.File["original"]; len(orig) > 0 {
		if up.orig, err = h.readCandidate(orig[0]); err != nil {
			up.close()

			return nil, err
		}
	}

	return up, nil
}

// uploadPlan is what an upload will do, decided before anything is stored.
type uploadPlan struct {
	app       *models.AndroidApp // nil when the package is new
	parent    *models.AndroidVersion
	storeOrig bool // the original part is stored as a new version
	warnings  []string
}

func conflict(message string, details map[string]any) error {
	return &conflictError{message: message, details: details}
}

// plan checks an upload against the repository: duplicates, signer policy and
// the original it derives from.
func (h *Handler) plan(ctx context.Context, repo *models.Repository, up *upload) (*uploadPlan, error) {
	pl := &uploadPlan{}
	pkg := up.main.info.Manifest.Package

	if prev, err := h.store.GetAndroidVersionBySHA256(ctx, repo.ID, up.main.sp.sha); err != nil {
		return nil, fmt.Errorf("failed to check for duplicates: %w", err)
	} else if prev != nil {
		return nil, conflict("this file is already stored", map[string]any{"duplicate": true, "id": prev.ID})
	}

	var err error
	if pl.app, err = h.store.GetAndroidApp(ctx, repo.ID, pkg); err != nil {
		return nil, fmt.Errorf("failed to look up app: %w", err)
	}

	if up.orig != nil {
		if err := h.planOriginal(ctx, repo, up, pl); err != nil {
			return nil, err
		}
	}

	if err := h.checkUploadSigner(ctx, repo.ID, pl, up.main, up.form.origin, up.form.acceptNewSigner); err != nil {
		return nil, err
	}

	if pl.parent == nil && !pl.storeOrig {
		if err := h.planParent(ctx, repo, up, pl); err != nil {
			return nil, err
		}
	}

	return pl, nil
}

// checkUploadSigner applies the app's signer policy to a candidate.
func (h *Handler) checkUploadSigner(ctx context.Context, repoID string, pl *uploadPlan, c *candidate, origin string, accept bool) error {
	appID, policy := "", models.AndroidPolicyWarn

	if pl.app != nil {
		appID, policy = pl.app.ID, pl.app.SignerPolicy
	}

	trust, known, err := h.store.AndroidSignerTrust(ctx, repoID, appID, c.signer())
	if err != nil {
		return fmt.Errorf("failed to look up signer: %w", err)
	}

	chk := checkSigner(policy, origin, trust, known, c.signer())
	if chk.Reject && !accept {
		return conflict("signer rejected by the app's policy: "+chk.Warning, map[string]any{
			"signer_sha256": c.signer(), "policy": policy, "origin": origin,
		})
	}

	if chk.Warning != "" {
		pl.warnings = append(pl.warnings, chk.Warning)
	}

	return nil
}

// planOriginal handles the original APK sent alongside a patched one.
func (h *Handler) planOriginal(ctx context.Context, repo *models.Repository, up *upload, pl *uploadPlan) error {
	if up.form.origin == models.AndroidOriginOriginal {
		return inputError("a file with origin=original cannot have an original part")
	}

	if up.orig.info.Manifest.Package != up.main.info.Manifest.Package {
		return inputErrorf("the original is package %q but the file is %q", up.orig.info.Manifest.Package, up.main.info.Manifest.Package)
	}

	existing, err := h.store.GetAndroidVersionBySHA256(ctx, repo.ID, up.orig.sp.sha)
	if err != nil {
		return fmt.Errorf("failed to look up the original: %w", err)
	}

	if existing != nil {
		pl.parent = existing

		return nil
	}

	pl.storeOrig = true

	return h.checkUploadSigner(ctx, repo.ID, pl, up.orig, models.AndroidOriginOriginal, up.form.acceptNewSigner)
}

// planParent resolves the parent given as a field, or finds the stored
// original of a patched file.
func (h *Handler) planParent(ctx context.Context, repo *models.Repository, up *upload, pl *uploadPlan) error {
	pkg, code := up.main.info.Manifest.Package, up.main.info.Manifest.VersionCode

	if ref := up.form.parent; ref != "" {
		var (
			p   *models.AndroidVersion
			err error
		)

		switch {
		case uuid.Validate(ref) == nil:
			p, err = h.store.GetAndroidVersion(ctx, repo.ID, ref, false)
		case sha256Re.MatchString(ref):
			p, err = h.store.GetAndroidVersionBySHA256(ctx, repo.ID, ref)
		default:
			return inputError("parent must be a version id or a sha256")
		}

		if err != nil {
			return fmt.Errorf("failed to look up the parent: %w", err)
		}

		if p == nil || p.PackageName != pkg {
			return inputError("parent is not a stored version of this package")
		}

		pl.parent = p

		return nil
	}

	if up.form.origin != models.AndroidOriginPatched || up.form.noOriginal {
		return nil
	}

	p, err := h.store.FindAndroidOriginal(ctx, repo.ID, pkg, code)
	if err != nil {
		return fmt.Errorf("failed to look up the original: %w", err)
	}

	if p == nil {
		pl.warnings = append(pl.warnings, "original not archived: send it in the original field or pass parent (no_original silences this)")

		return nil
	}

	pl.parent = p

	return nil
}

// buildVersion fills a version from an inspected APK.
func buildVersion(repo *models.Repository, app *models.AndroidApp, c *candidate, blobID, origin string, f *uploadForm, warnings []string, createdBy string) (*models.AndroidVersion, error) {
	info := c.info
	m := &info.Manifest

	entries := info.Entries
	if len(entries) > maxEntriesStored {
		entries = entries[:maxEntriesStored]
		warnings = append(warnings, fmt.Sprintf("only the first %d of %d zip entries are listed", maxEntriesStored, len(info.Entries)))
	}

	all := append(slicesClone(info.Warnings), warnings...)

	v := &models.AndroidVersion{
		RepositoryID: repo.ID, AppID: app.ID, PackageName: m.Package, VersionCode: m.VersionCode, VersionName: m.VersionName,
		FileKind: "apk", BlobID: blobID, SHA256: c.sp.sha, SizeBytes: c.sp.size, Filename: c.filename,
		SignerSHA256: info.SignerSHA256(), ABIs: info.ABIs, VariantKey: variantKey(info.SignerSHA256(), info.ABIs),
		Label: info.Label, LauncherLabel: info.LauncherLabel,
		MinSDK: m.MinSDK, TargetSDK: m.TargetSDK, CompileSDK: m.CompileSDK, MaxSDK: m.MaxSDK, Debuggable: m.Application.Debuggable,
		Locales: info.Locales, Frameworks: info.Frameworks, SigSchemes: info.Signature.Schemes, SigVerified: info.Signature.Verified,
		ContentDigest: info.ContentDigest, PageSize16K: info.PageSize16K, ManifestXML: info.ManifestXML,
		Origin: origin, PatchInfo: f.patchInfo, SourceURL: f.sourceURL, Changelog: f.changelog, Notes: f.notes,
		Tags: f.tags, Channel: f.channel, CreatedBy: createdBy,
	}

	for dst, src := range map[*json.RawMessage]any{
		&v.Labels: info.Labels, &v.Manifest: info.Manifest, &v.Signature: info.Signature, &v.NativeLibs: info.NativeLibs,
		&v.Dependencies: info.Dependencies, &v.BuildInfo: info.BuildInfo, &v.ZipChecks: info.ZipChecks,
		&v.Entries: entries, &v.Warnings: all,
	} {
		raw, err := json.Marshal(src)
		if err != nil {
			return nil, fmt.Errorf("failed to encode APK metadata: %w", err)
		}

		*dst = raw
	}

	return v, nil
}

func slicesClone(s []string) []string { return append([]string(nil), s...) }

// persisted is what an upload stored.
type persisted struct {
	main *models.AndroidVersion
	orig *models.AndroidVersion // the original stored by this upload, if any
}

// blobPut tracks the blobs an upload newly wrote so a failure can remove them.
type blobPut struct {
	h       *Handler
	repo    string
	created []string
}

// put stores content under its sha256 unless the store already has it and
// records the blobs row; it returns the blob UUID.
func (b *blobPut) put(ctx context.Context, sha string, size int64, content io.Reader) (string, error) {
	_, exists, err := b.h.writeBlob.ExistsBySHA256(ctx, sha)
	if err != nil {
		return "", fmt.Errorf("failed to check blob existence: %w", err)
	}

	if !exists {
		if _, err := b.h.writeBlob.Put(ctx, content, map[string]string{"sha256": sha, "format": Format, "repo_name": b.repo}); err != nil {
			return "", fmt.Errorf("failed to store blob: %w", err)
		}

		b.created = append(b.created, sha)
	}

	id, err := b.h.DB.UpsertBlob(ctx, sha, sha, size)
	if err != nil {
		return "", fmt.Errorf("failed to record blob: %w", err)
	}

	return id, nil
}

// putFile stores a buffered upload.
func (b *blobPut) putFile(ctx context.Context, c *candidate) (string, error) {
	if _, err := c.sp.f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to rewind upload: %w", err)
	}

	return b.put(ctx, c.sp.sha, c.sp.size, c.sp.f)
}

// iconRows stores the icon images of c and returns their rows.
func (b *blobPut) iconRows(ctx context.Context, c *candidate) ([]models.AndroidIcon, error) {
	rows := make([]models.AndroidIcon, 0, len(c.info.Icons))

	for _, ic := range c.info.Icons {
		row := models.AndroidIcon{
			Role: ic.Role, Density: ic.Density, Width: ic.Width, Height: ic.Height, Mime: ic.Mime, SourcePath: ic.SourcePath,
			ResourceName: ic.ResourceName, Color: ic.Color, Adaptive: ic.Adaptive, Renderable: ic.Renderable, SizeBytes: ic.Size,
		}

		if len(ic.Data) > 0 {
			sum := sha256.Sum256(ic.Data)
			row.SHA256 = hex.EncodeToString(sum[:])
			row.SizeBytes = int64(len(ic.Data))

			id, err := b.put(ctx, row.SHA256, row.SizeBytes, bytes.NewReader(ic.Data))
			if err != nil {
				return nil, err
			}

			row.BlobID = id
		}

		rows = append(rows, row)
	}

	return rows, nil
}

func iconShas(c *candidate) []string {
	var out []string

	for _, ic := range c.info.Icons {
		if len(ic.Data) > 0 {
			sum := sha256.Sum256(ic.Data)
			out = append(out, hex.EncodeToString(sum[:]))
		}
	}

	return out
}

// persist stores the blobs and rows of an upload.
func (h *Handler) persist(ctx context.Context, repo *models.Repository, up *upload, pl *uploadPlan, createdBy string) (*persisted, error) {
	release := func(size int64) {
		if err := quota.Release(ctx, h.DB, repo.ID, size); err != nil {
			h.Log.Warnw("failed to release quota", "repository", repo.Name, "error", err)
		}
	}

	if err := quota.Enforce(ctx, h.DB, repo.ID, up.main.sp.size); err != nil {
		return nil, err //nolint:wrapcheck // callers compare with quota.ErrExceeded
	}

	shas := append([]string{up.main.sp.sha}, iconShas(up.main)...)

	if pl.storeOrig {
		if err := quota.Enforce(ctx, h.DB, repo.ID, up.orig.sp.size); err != nil {
			release(up.main.sp.size)

			return nil, err //nolint:wrapcheck // callers compare with quota.ErrExceeded
		}

		shas = append(append(shas, up.orig.sp.sha), iconShas(up.orig)...)
	}

	var (
		res       = &persisted{}
		puts      = &blobPut{h: h, repo: repo.Name}
		duplicate bool
	)

	err := storage.WithBlobLocks(ctx, h.DB, uniqueStrings(shas), func(ctx context.Context) error {
		var err error

		duplicate, err = h.insertRows(ctx, repo, up, pl, puts, createdBy, res)

		return err
	})

	// The original keeps its reservation only if this upload created its row.
	if pl.storeOrig && res.orig == nil {
		release(up.orig.sp.size)
	}

	if err != nil || duplicate {
		release(up.main.sp.size)
		h.dropBlobs(ctx, puts.created)

		if err != nil {
			return nil, err
		}

		return nil, conflict(errVariantStored, map[string]any{"duplicate": true})
	}

	return res, nil
}

const errVariantStored = "a file with the same package, versionCode, signer and ABIs is already stored"

// insertRows runs under the blob locks: it stores blobs and creates the app
// and version rows, recording in res what it created. duplicate is true when
// the main file's variant exists.
func (h *Handler) insertRows(ctx context.Context, repo *models.Repository, up *upload, pl *uploadPlan, puts *blobPut, createdBy string, res *persisted) (bool, error) {
	app, err := h.store.UpsertAndroidApp(ctx, repo.ID, up.main.info.Manifest.Package)
	if err != nil {
		return false, fmt.Errorf("failed to create app: %w", err)
	}

	parent := pl.parent

	if pl.storeOrig {
		if parent, err = h.storeOriginal(ctx, repo, app, up, puts, createdBy, res); err != nil {
			return false, err
		}
	}

	main, err := h.insertVersion(ctx, repo, app, up.main, up.form.origin, up.form, pl.warnings, createdBy, puts, parent)
	if errors.Is(err, database.ErrDuplicate) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if up.form.origin == models.AndroidOriginOriginal {
		if err := h.registerOriginalSigner(ctx, repo.ID, app, up.main); err != nil {
			return false, err
		}
	}

	res.main = main

	return false, nil
}

// storeOriginal inserts the original part of an upload and returns the version
// the main file should point to. res.orig is set only when this call created
// the row.
func (h *Handler) storeOriginal(ctx context.Context, repo *models.Repository, app *models.AndroidApp, up *upload, puts *blobPut, createdBy string, res *persisted) (*models.AndroidVersion, error) {
	orig, err := h.insertVersion(ctx, repo, app, up.orig, models.AndroidOriginOriginal, &uploadForm{}, nil, createdBy, puts, nil)
	if errors.Is(err, database.ErrDuplicate) {
		// The same file may have been stored meanwhile by a concurrent upload.
		stored, err := h.store.GetAndroidVersionBySHA256(ctx, repo.ID, up.orig.sp.sha)
		if err != nil {
			return nil, fmt.Errorf("failed to look up the original: %w", err)
		}

		if stored == nil {
			return nil, conflict("the original: "+errVariantStored, map[string]any{"duplicate": true})
		}

		return stored, nil
	}

	if err != nil {
		return nil, err
	}

	res.orig = orig

	if err := h.registerOriginalSigner(ctx, repo.ID, app, up.orig); err != nil {
		return nil, err
	}

	return orig, nil
}

// insertVersion stores the blobs of c and inserts its version row, linked to parent when non-nil.
func (h *Handler) insertVersion(ctx context.Context, repo *models.Repository, app *models.AndroidApp, c *candidate, origin string, f *uploadForm, warnings []string, createdBy string, puts *blobPut, parent *models.AndroidVersion) (*models.AndroidVersion, error) {
	blobID, err := puts.putFile(ctx, c)
	if err != nil {
		return nil, err
	}

	icons, err := puts.iconRows(ctx, c)
	if err != nil {
		return nil, err
	}

	v, err := buildVersion(repo, app, c, blobID, origin, f, warnings, createdBy)
	if err != nil {
		return nil, err
	}

	if parent != nil {
		v.ParentID, v.OriginalSHA256 = parent.ID, parent.SHA256
	}

	if err := h.store.CreateAndroidVersion(ctx, v, icons); err != nil {
		return nil, err //nolint:wrapcheck // callers compare with database.ErrDuplicate
	}

	return v, nil
}

// registerOriginalSigner records the publisher's key of an original unless the
// repository already knows it (a key marked as yours is never downgraded).
func (h *Handler) registerOriginalSigner(ctx context.Context, repoID string, app *models.AndroidApp, c *candidate) error {
	signer := c.signer()
	if signer == "" {
		return nil
	}

	if _, known, err := h.store.AndroidSignerTrust(ctx, repoID, app.ID, signer); err != nil || known {
		return err //nolint:wrapcheck // wrapped by the caller's context
	}

	cert, err := json.Marshal(c.info.Signature.Signer)
	if err != nil {
		return fmt.Errorf("failed to encode the original's certificate: %w", err)
	}

	if err := h.store.UpsertAndroidSigner(ctx, &models.AndroidSigner{
		RepositoryID: repoID, SHA256: signer, Trust: models.AndroidTrustOriginal, AppID: app.ID, Certificate: cert,
	}); err != nil {
		return fmt.Errorf("failed to record the original's signer: %w", err)
	}

	return nil
}

// dropBlobs removes blobs a failed upload wrote, unless something else uses them.
func (h *Handler) dropBlobs(ctx context.Context, shas []string) {
	for _, sha := range uniqueStrings(shas) {
		if err := storage.SafeDeleteBlob(context.WithoutCancel(ctx), h.DB, h.writeBlob, sha); err != nil {
			h.Log.Warnw("failed to clean up blob after failed upload", "sha256", sha, "error", err)
		}
	}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))

	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	return out
}

func (h *Handler) writeUploadError(w http.ResponseWriter, err error, what string) {
	var ce *conflictError

	switch msg, isInput := isInputError(err); {
	case errors.Is(err, errTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "INVALID_INPUT", "file exceeds the maximum upload size")
	case isInput:
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", msg)
	case errors.As(err, &ce):
		writeErrorDetails(w, http.StatusConflict, "CONFLICT", ce.message, ce.details)
	case errors.Is(err, quota.ErrExceeded):
		quota.WriteExceeded(w)
	default:
		h.Log.Errorw(what+" failed", "error", err)
		writeError(w, http.StatusInternalServerError, "UPLOAD_ERROR", what+" failed")
	}
}

func principalName(r *http.Request) string {
	if p := middleware.PrincipalFromContext(r.Context()); p != nil {
		return p.Username
	}

	return ""
}

// handleUpload stores an APK, optionally with the original it was patched from.
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	result := "success"

	if h.reg != nil {
		defer func() { h.reg.UploadsTotal.WithLabelValues(Format, result).Inc() }()
	}

	repo := h.repoOf(w, r, true)
	if repo == nil {
		result = "error"

		return
	}

	up, err := h.readUpload(w, r)
	if err != nil {
		result = "error"

		h.writeUploadError(w, err, "upload processing")

		return
	}
	defer up.close()

	pl, err := h.plan(r.Context(), repo, up)
	if err == nil {
		var res *persisted

		if res, err = h.persist(r.Context(), repo, up, pl, principalName(r)); err == nil {
			h.finishUpload(w, r, repo, up, pl, res)

			return
		}
	}

	result = "error"

	h.writeUploadError(w, err, "upload processing")
}

func (h *Handler) finishUpload(w http.ResponseWriter, r *http.Request, repo *models.Repository, up *upload, pl *uploadPlan, res *persisted) {
	v := res.main
	m := &up.main.info.Manifest

	for _, stored := range []*models.AndroidVersion{res.orig, v} {
		if stored == nil {
			continue
		}

		h.Emit(r.Context(), repo.ID, webhook.EventArtifactUploaded, webhook.ArtifactPayload(webhook.EventArtifactUploaded, webhook.Artifact{
			Repository: repo.Name, Format: Format, Name: stored.PackageName, Version: stored.VersionName,
			Path: stored.Filename, SHA256: stored.SHA256, Size: stored.SizeBytes,
		}))
	}

	h.Log.Infow("Android package uploaded", "repository", repo.Name, "package", m.Package, "version_code", m.VersionCode, "signer", v.SignerSHA256)

	out := map[string]any{
		"id": v.ID, "package_name": v.PackageName, "version_code": v.VersionCode, "version_name": v.VersionName,
		"sha256": v.SHA256, "size_bytes": v.SizeBytes, "signer_sha256": v.SignerSHA256, "origin": v.Origin,
		"parent_id": v.ParentID, "filename": v.Filename, "warnings": append([]string{}, pl.warnings...),
	}

	if res.orig != nil {
		out["original_id"] = res.orig.ID
	}

	writeOK(w, http.StatusCreated, out)
}

// handleInspect analyses an APK without storing it.
func (h *Handler) handleInspect(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload+multipartOverhead)

	if err := r.ParseMultipartForm(multipartMemory); err != nil { //nolint:gosec // r.Body is bounded by http.MaxBytesReader above
		h.writeUploadError(w, inputErrorf("failed to parse form: %v", err), "inspect")

		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		h.writeUploadError(w, inputError("send exactly one APK in the file field"), "inspect")

		return
	}

	c, err := h.readCandidate(files[0])
	if err != nil {
		h.writeUploadError(w, err, "inspect")

		return
	}
	defer c.close()

	out := map[string]any{
		"info": c.info, "sha256": c.sp.sha, "size_bytes": c.sp.size, "filename": c.filename,
		"duplicate": nil, "previous": nil,
	}

	if dup, err := h.store.GetAndroidVersionBySHA256(r.Context(), repo.ID, c.sp.sha); err != nil {
		h.fail(w, "failed to check for duplicates", err)

		return
	} else if dup != nil {
		out["duplicate"] = dup.ID
	}

	prev, err := h.store.LatestAndroidVersion(r.Context(), repo.ID, c.info.Manifest.Package, database.AndroidLatestFilter{})
	if err != nil {
		h.fail(w, "failed to look up the previous version", err)

		return
	}

	if prev != nil {
		out["previous"] = prev
	}

	writeOK(w, http.StatusOK, out)
}
