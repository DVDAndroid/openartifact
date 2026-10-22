// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/format/base"
	"github.com/openartifact/openartifact/internal/presign"
	"github.com/openartifact/openartifact/internal/quarantine"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/apk"
)

const (
	apkContentType = "application/vnd.android.package-archive"
	// metadataMaxAge is how long a shared cache may keep latest.json and index.json of a public repository.
	metadataMaxAge = 60
)

var abiRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// clientVersion is what update checkers read: latest.json and the entries of index.json.
type clientVersion struct {
	PackageName  string    `json:"package_name"`
	VersionName  string    `json:"version_name"`
	VersionCode  int64     `json:"version_code"`
	SHA256       string    `json:"sha256"`
	SignerSHA256 string    `json:"signer_sha256"`
	URL          string    `json:"url"`
	Size         int64     `json:"size"`
	Changelog    string    `json:"changelog"`
	Origin       string    `json:"origin"`
	Channel      string    `json:"channel"`
	MinSDK       int       `json:"min_sdk"`
	TargetSDK    int       `json:"target_sdk"`
	ABIs         []string  `json:"abis"`
	CreatedAt    time.Time `json:"created_at"`
}

// repoPath is the path of the client routes of a repository.
func repoPath(name string) string { return "/repos/" + name }

// downloadPath is the path a version is downloaded from.
func downloadPath(repoName string, v *models.AndroidVersion) string {
	return repoPath(repoName) + "/apps/" + v.PackageName + "/" + strconv.FormatInt(v.VersionCode, 10) + "/" + v.Filename
}

func (h *Handler) clientVersion(repoName string, v *models.AndroidVersion) clientVersion {
	return clientVersion{
		PackageName: v.PackageName, VersionName: v.VersionName, VersionCode: v.VersionCode, SHA256: v.SHA256,
		SignerSHA256: v.SignerSHA256, URL: h.BaseURL + downloadPath(repoName, v), Size: v.SizeBytes, Changelog: v.Changelog,
		Origin: v.Origin, Channel: v.Channel, MinSDK: v.MinSDK, TargetSDK: v.TargetSDK, ABIs: append([]string{}, v.ABIs...),
		CreatedAt: v.CreatedAt,
	}
}

// handleDownload serves one stored file.
func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	vars := urlparams.Vars(r)

	code, err := strconv.ParseInt(vars["code"], 10, 64)
	if err != nil || code < 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "file not found")

		return
	}

	v, err := h.store.GetAndroidDownload(r.Context(), repo.ID, vars["pkg"], code, vars["filename"])
	if err != nil {
		h.fail(w, "failed to look up the file", err)

		return
	}

	if v == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "file not found")

		return
	}

	h.serveVersion(w, r, repo, v)
}

func (h *Handler) serveVersion(w http.ResponseWriter, r *http.Request, repo *models.Repository, v *models.AndroidVersion) {
	if h.DenyIfQuarantined(w, r, repo.ID, quarantine.PathKeys(downloadPath(repo.Name, v), r.URL.Path)...) {
		return
	}

	hdr := w.Header()
	hdr.Set("Content-Type", apkContentType)
	hdr.Set("Content-Disposition", `attachment; filename="`+v.Filename+`"`)
	hdr.Set("X-Checksum-Sha256", v.SHA256)
	hdr.Set("X-OA-APK-Signer", v.SignerSHA256)

	if base.Revalidate(w, r, repo.Public, `"`+v.SHA256+`"`, metadataMaxAge) {
		return
	}

	if r.Method == http.MethodHead {
		hdr.Set("Content-Length", strconv.FormatInt(v.SizeBytes, 10))
		w.WriteHeader(http.StatusOK)

		return
	}

	if err := h.store.IncrementAndroidDownload(r.Context(), v.ID); err != nil {
		h.Log.Warnw("failed to count android download", "id", v.ID, "error", err)
	}

	if h.TryPresign(w, r, presign.Blob{SHA256: v.SHA256, Size: v.SizeBytes, Filename: v.Filename, ContentType: apkContentType, Repository: repo}) {
		return
	}

	rc, err := h.BlobStore.Get(r.Context(), v.SHA256)
	if err != nil {
		h.Log.Errorw("failed to read android blob", "sha256", v.SHA256, "error", err)
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to read the file")

		return
	}
	defer base.CloseLog(rc, h.Log)

	hdr.Set("Content-Length", strconv.FormatInt(v.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		h.Log.Warnw("android download interrupted", "id", v.ID, "error", err)
	}
}

// latestFilter reads the query of latest.apk and latest.json.
func latestFilter(r *http.Request) (database.AndroidLatestFilter, error) {
	q := r.URL.Query()

	var f database.AndroidLatestFilter

	if s := q.Get("sdk"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 1000 {
			return f, inputError("sdk must be an API level")
		}

		f.SDK = n
	}

	if f.ABI = q.Get("abi"); f.ABI != "" && !abiRe.MatchString(f.ABI) {
		return f, inputError("invalid abi")
	}

	var err error
	if f.Channel, err = normalizeChannel(q.Get("channel")); err != nil {
		return f, err
	}

	if s := q.Get("signer"); s != "" {
		sig, ok := apk.NormalizeFingerprint(s)
		if !ok {
			return f, inputError("signer must be a SHA-256 fingerprint")
		}

		f.Signer = sig
	}

	if f.Origin = q.Get("origin"); f.Origin != "" && !validOrigin(f.Origin) {
		return f, inputError("origin must be one of built, patched, original, unknown")
	}

	return f, nil
}

// latest resolves the newest version for the request, answering the errors itself.
func (h *Handler) latest(w http.ResponseWriter, r *http.Request) (*models.Repository, *models.AndroidVersion) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return nil, nil
	}

	f, err := latestFilter(r)
	if err != nil {
		msg, _ := isInputError(err)
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", msg)

		return nil, nil
	}

	v, err := h.store.LatestAndroidVersion(r.Context(), repo.ID, urlparams.Vars(r)["pkg"], f)
	if err != nil {
		h.fail(w, "failed to look up the latest version", err)

		return nil, nil
	}

	if v == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "no matching version")

		return nil, nil
	}

	return repo, v
}

// handleLatestAPK redirects to the file of the newest matching version.
func (h *Handler) handleLatestAPK(w http.ResponseWriter, r *http.Request) {
	repo, v := h.latest(w, r)
	if v == nil {
		return
	}

	w.Header().Set("Cache-Control", "private, no-cache")
	http.Redirect(w, r, h.BaseURL+downloadPath(repo.Name, v), http.StatusFound)
}

// handleLatestJSON describes the newest matching version for update checkers.
func (h *Handler) handleLatestJSON(w http.ResponseWriter, r *http.Request) {
	repo, v := h.latest(w, r)
	if v == nil {
		return
	}

	h.writeMetadata(w, r, repo, h.clientVersion(repo.Name, v))
}

// handleIndexJSON lists every stored version of a package.
func (h *Handler) handleIndexJSON(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	pkg := urlparams.Vars(r)["pkg"]

	app, err := h.store.GetAndroidApp(r.Context(), repo.ID, pkg)
	if err != nil {
		h.fail(w, "failed to look up the app", err)

		return
	}

	if app == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "app not found")

		return
	}

	vs, _, err := h.store.ListAndroidVersions(r.Context(), repo.ID, models.AndroidVersionFilter{AppID: app.ID, Limit: 500})
	if err != nil {
		h.fail(w, "failed to list versions", err)

		return
	}

	out := struct {
		PackageName string          `json:"package_name"`
		Versions    []clientVersion `json:"versions"`
	}{PackageName: pkg, Versions: make([]clientVersion, 0, len(vs))}

	for i := range vs {
		out.Versions = append(out.Versions, h.clientVersion(repo.Name, &vs[i]))
	}

	if len(out.Versions) == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "app not found")

		return
	}

	h.writeMetadata(w, r, repo, out)
}

// writeMetadata answers a JSON document with an ETag derived from its content.
func (h *Handler) writeMetadata(w http.ResponseWriter, r *http.Request, repo *models.Repository, doc any) {
	body, err := json.Marshal(doc)
	if err != nil {
		h.fail(w, "failed to encode the answer", err)

		return
	}

	sum := sha256.Sum256(body)

	if base.Revalidate(w, r, repo.Public, `"`+hex.EncodeToString(sum[:16])+`"`, metadataMaxAge) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if r.Method != http.MethodHead {
		_, _ = w.Write(append(body, '\n'))
	}
}
