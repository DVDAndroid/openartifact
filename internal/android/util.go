// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	maxNoteBytes = 64 << 10
	maxTags      = 32
	maxTagLen    = 64
	maxURLLen    = 2048
	// maxJSONBody bounds the JSON bodies of the management API.
	maxJSONBody = 1 << 20
)

var (
	packageNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	tagRe         = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]*$`)
	channelRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	sha256Re      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// inputError is a problem with what the client sent; its text is shown to the client.
type inputError string

func (e inputError) Error() string { return string(e) }

func inputErrorf(format string, args ...any) error { return inputError(fmt.Sprintf(format, args...)) }

// isInputError reports the client-facing message of err when it is an inputError.
func isInputError(err error) (string, bool) {
	if ie, ok := errors.AsType[inputError](err); ok {
		return string(ie), true
	}

	return "", false
}

// response is the envelope of successful answers, as in internal/api.
type response struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
}

// errorResponse is the envelope of failures, as in internal/api.
type errorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The header is already written; an encoding failure cannot change the answer.
	_ = json.NewEncoder(w).Encode(v) //nolint:errchkjson // v is a response struct built in this package
}

func writeOK(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, response{Success: true, Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: message, Code: code})
}

func writeErrorDetails(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, errorResponse{Error: message, Code: code, Details: details})
}

// decodeJSON reads a bounded JSON object body into dst.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return inputErrorf("invalid JSON body: %v", err)
	}

	return nil
}

// validPackageName reports whether s is usable as an Android package name. The
// name comes from an untrusted manifest and ends up in URLs and file names.
func validPackageName(s string) bool {
	return len(s) > 0 && len(s) <= 255 && packageNameRe.MatchString(s)
}

// normalizeTags lowercases, deduplicates and validates tags.
func normalizeTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))

	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}

		if len(t) > maxTagLen || !tagRe.MatchString(t) {
			return nil, inputErrorf("invalid tag %q: use lowercase letters, digits and . _ : - (at most %d bytes)", t, maxTagLen)
		}

		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}

	if len(out) > maxTags {
		return nil, inputErrorf("too many tags (at most %d)", maxTags)
	}

	return out, nil
}

// splitTags splits a comma separated tag list.
func splitTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	return strings.Split(s, ",")
}

func validOrigin(s string) bool {
	switch s {
	case "built", "patched", "original", "unknown":
		return true
	}

	return false
}

func normalizeChannel(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s != "" && !channelRe.MatchString(s) {
		return "", inputErrorf("invalid channel %q", s)
	}

	return s, nil
}

func checkText(name, s string) error {
	if len(s) > maxNoteBytes {
		return inputErrorf("%s is longer than %d bytes", name, maxNoteBytes)
	}

	return nil
}

// checkSourceURL accepts an empty string or an http(s) URL. The value is only
// stored and shown as a link; the server never fetches it.
func checkSourceURL(s string) error {
	if s == "" {
		return nil
	}

	u, err := url.Parse(s)
	if err != nil || len(s) > maxURLLen || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return inputError("source_url must be an http(s) URL")
	}

	return nil
}

// checkPatchInfo accepts an empty value or a JSON object of reasonable size.
func checkPatchInfo(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}

	var obj map[string]any
	if len(raw) > maxNoteBytes || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return inputErrorf("patch_info must be a JSON object of at most %d bytes", maxNoteBytes)
	}

	return nil
}

// variantKey identifies a file among those sharing a package and versionCode:
// the signer and the set of ABIs (a patched APK keeps the original's
// versionCode, and split-by-ABI builds share one too).
func variantKey(signer string, abis []string) string {
	sorted := slices.Clone(abis)
	slices.Sort(sorted)

	sum := sha256.Sum256([]byte(signer + "\n" + strings.Join(sorted, ",")))

	return hex.EncodeToString(sum[:])
}

// downloadName is the file name a version is served under; it carries the
// signer so variants of one versionCode have distinct URLs.
func downloadName(pkg string, versionCode int64, signer string) string {
	tag := "unsigned"
	if len(signer) >= 8 {
		tag = signer[:8]
	}

	return pkg + "-" + strconv.FormatInt(versionCode, 10) + "-" + tag + ".apk"
}

// spooled is an upload buffered on disk with its digest.
type spooled struct {
	f    *os.File
	sha  string
	size int64
}

// spool copies src to a temporary file, hashing it and refusing more than limit bytes.
func spool(src io.Reader, limit int64) (*spooled, error) {
	f, err := os.CreateTemp("", "android-upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	h := sha256.New()

	n, err := io.Copy(f, io.TeeReader(io.LimitReader(src, limit+1), h))
	if err != nil {
		discard(f)

		return nil, fmt.Errorf("failed to buffer upload: %w", err)
	}

	if n > limit {
		discard(f)

		return nil, errTooLarge
	}

	return &spooled{f: f, sha: hex.EncodeToString(h.Sum(nil)), size: n}, nil
}

var errTooLarge = errors.New("file exceeds the maximum upload size")

// discard closes and removes a temporary file this package created.
func discard(f *os.File) {
	_ = f.Close()
	_ = os.Remove(f.Name()) //nolint:gosec // f.Name() is the path os.CreateTemp returned, not user input
}

func (s *spooled) close() {
	if s != nil && s.f != nil {
		discard(s.f)
	}
}

// pageParams reads page and limit (defaults 1 and def, at most 200) and
// returns them with the row offset.
func pageParams(r *http.Request, def int) (int, int, int) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	page = max(page, 1)

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 {
		limit = def
	}

	limit = min(limit, 200)

	return page, limit, (page - 1) * limit
}

// queryList reads a repeated or comma separated query parameter.
func queryList(r *http.Request, key string) []string {
	var out []string

	for _, v := range r.URL.Query()[key] {
		out = append(out, splitTags(v)...)
	}

	return out
}
