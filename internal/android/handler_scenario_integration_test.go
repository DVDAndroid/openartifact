// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package android

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/openartifact/openartifact/pkg/apk/apktest"
)

const (
	apiBase  = "/api/v1/repositories/apps/apks"
	filePath = "/repos/apps/apps/org.example.app/"
)

// scenario is one repository's life: an original from its publisher, a copy
// patched and re-signed by its owner, and what a client and the UI do with them.
// Each step relies on the ones before it.
type scenario struct {
	t                 *testing.T // the running step's
	e                 *env
	publisher, mine   *apktest.Key
	original          []byte
	patched           []byte
	origID, patchedID string
}

func (s *scenario) fileName(code int, key *apktest.Key) string {
	return "org.example.app-" + strconv.Itoa(code) + "-" + key.SHA256()[:8] + ".apk"
}

func TestAndroidUploadAndServe(t *testing.T) {
	e := newEnv(t)
	e.repo("apps", "android")
	e.repo("other", "android")
	e.repo("plain", "raw")

	s := &scenario{e: e, publisher: apktest.NewRSAKey("Publisher"), mine: apktest.NewRSAKey("Me")}
	s.original, s.patched = demoAPK(10, s.publisher), demoAPK(10, s.mine)

	for _, step := range []struct {
		name string
		run  func()
	}{
		{"an original registers its publisher key", s.uploadOriginal},
		{"a duplicate is a conflict naming the stored version", s.duplicate},
		{"a patched file links to the original and warns about the key", s.uploadPatched},
		{"version detail tells a re-signed copy from a modified one", s.detail},
		{"marking the key as mine silences the warning", s.markMine},
		{"clients download and revalidate", s.download},
		{"clients resolve latest and read the index", s.latestAndIndex},
		{"icons are served inert and the best one fits the size", s.icons},
		{"versions are scoped to their repository and format", s.scoping},
		{"bad uploads are refused", s.badUploads},
		{"the reject policy needs an explicit accept", s.rejectPolicy},
		{"version metadata is validated and stored", s.versionMeta},
		{"listing, tags and signer use", s.listing},
		{"attached files follow the version and can be removed", s.attachments},
		{"deleting a version removes its download and blob", s.deletion},
	} {
		t.Run(step.name, func(t *testing.T) {
			s.t = t
			step.run()
		})
	}
}

func (s *scenario) uploadOriginal() {
	d := s.e.upload("apps", map[string]string{"origin": "original", "tags": "Base"}, part{"file", "orig.apk", s.original}).data(s.t, http.StatusCreated)

	s.origID, _ = d["id"].(string)
	if d["package_name"] != "org.example.app" || d["signer_sha256"] != s.publisher.SHA256() || d["version_code"].(float64) != 10 {
		s.t.Fatalf("upload = %v", d)
	}

	signers := s.e.getJSON(apiBase + "/signers").Body.String()
	if !strings.Contains(signers, s.publisher.SHA256()) || !strings.Contains(signers, `"original"`) {
		s.t.Fatalf("publisher key not registered as original: %s", signers)
	}
}

func (s *scenario) duplicate() {
	r := s.e.upload("apps", map[string]string{"origin": "original"}, part{"file", "orig.apk", s.original})
	r.wantStatus(s.t, http.StatusConflict)

	if !strings.Contains(r.Body.String(), s.origID) {
		s.t.Fatalf("conflict does not name %s: %s", s.origID, r.Body.String())
	}
}

func (s *scenario) uploadPatched() {
	d := s.e.upload("apps", map[string]string{"origin": "patched", "tags": "revanced, Device:Pixel", "patch_info": `{"tool":"revanced"}`},
		part{"file", "patched.apk", s.patched}, part{"original", "orig.apk", s.original}).data(s.t, http.StatusCreated)

	s.patchedID, _ = d["id"].(string)
	if d["parent_id"] != s.origID {
		s.t.Fatalf("parent_id = %v, want %s", d["parent_id"], s.origID)
	}

	if w, _ := d["warnings"].([]any); len(w) != 1 || !strings.Contains(w[0].(string), "not signed with a key marked as yours") {
		s.t.Fatalf("warnings = %v", d["warnings"])
	}
}

func (s *scenario) detail() {
	d := s.e.getJSON(apiBase+"/versions/"+s.patchedID).data(s.t, http.StatusOK)

	parent, _ := d["parent"].(map[string]any)
	if parent["same_signer"] != false || parent["same_content"] != true {
		s.t.Fatalf("parent = %v", parent)
	}

	v := d["version"].(map[string]any)
	if v["origin"] != "patched" || v["label"] != "Demo" || v["download_url"] == "" {
		s.t.Fatalf("version = %v", v)
	}

	tags, _ := v["tags"].([]any)
	if len(tags) != 2 || tags[1] != "device:pixel" {
		s.t.Fatalf("tags = %v", tags)
	}

	if icons, _ := d["icons"].([]any); len(icons) != 2 {
		s.t.Fatalf("icons = %v", d["icons"])
	}
}

func (s *scenario) markMine() {
	s.e.postJSON(apiBase+"/signers", map[string]any{"sha256": strings.ToUpper(s.mine.SHA256()), "trust": "mine", "label": "release"}).wantStatus(s.t, http.StatusCreated)

	d := s.e.upload("apps", map[string]string{"origin": "patched"}, part{"file", "p11.apk", demoAPK(11, s.mine)}).data(s.t, http.StatusCreated)
	if w, _ := d["warnings"].([]any); len(w) != 1 || !strings.Contains(w[0].(string), "original not archived") {
		s.t.Fatalf("warnings = %v (want only the missing original)", d["warnings"])
	}
}

func (s *scenario) download() {
	path := filePath + "10/" + s.fileName(10, s.mine)

	dl := s.e.getJSON(path)
	dl.wantStatus(s.t, http.StatusOK)

	if !bytes.Equal(dl.Body.Bytes(), s.patched) || dl.Header().Get("X-OA-APK-Signer") != s.mine.SHA256() ||
		dl.Header().Get("Content-Type") != apkContentType {
		s.t.Fatalf("download differs: %d bytes, headers %v", dl.Body.Len(), dl.Header())
	}

	head := s.e.do(http.MethodHead, path, nil, "")
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(s.patched)) {
		s.t.Fatalf("HEAD = %d, %d body bytes, length %q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}

	req := httptest.NewRequestWithContext(s.t.Context(), http.MethodGet, path, nil)
	req.Header.Set("If-None-Match", dl.Header().Get("ETag"))

	rec := httptest.NewRecorder()
	s.e.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		s.t.Fatalf("conditional GET = %d", rec.Code)
	}

	// Another signer's name is another file, never a silent substitute.
	s.e.getJSON(filePath+"10/org.example.app-10-deadbeef.apk").wantStatus(s.t, http.StatusNotFound)
	s.e.getJSON(filePath+"99/x.apk").wantStatus(s.t, http.StatusNotFound)
}

func (s *scenario) latestAndIndex() {
	latest := s.e.getJSON(filePath + "latest.apk")
	if latest.Code != http.StatusFound || !strings.HasSuffix(latest.Header().Get("Location"), "/org.example.app/11/"+s.fileName(11, s.mine)) {
		s.t.Fatalf("latest.apk = %d -> %q", latest.Code, latest.Header().Get("Location"))
	}

	var lj clientVersion

	r := s.e.getJSON(filePath + "latest.json?signer=" + s.publisher.SHA256())
	r.wantStatus(s.t, http.StatusOK)

	if err := json.Unmarshal(r.Body.Bytes(), &lj); err != nil || lj.VersionCode != 10 || lj.Origin != "original" ||
		!strings.HasPrefix(lj.URL, "http://oa.test"+filePath+"10/") {
		s.t.Fatalf("latest.json = %+v, %v", lj, err)
	}

	s.e.getJSON(filePath+"latest.json?sdk=19").wantStatus(s.t, http.StatusNotFound)
	s.e.getJSON(filePath+"latest.json?signer=zz").wantStatus(s.t, http.StatusBadRequest)

	idx := s.e.getJSON(filePath + "index.json")
	idx.wantStatus(s.t, http.StatusOK)

	if n := strings.Count(idx.Body.String(), `"version_code"`); n != 3 {
		s.t.Fatalf("index.json lists %d versions, want 3: %s", n, idx.Body.String())
	}
}

func (s *scenario) icons() {
	width := func(size string) int {
		r := s.e.getJSON(apiBase + "/versions/" + s.patchedID + "/icon?size=" + size)
		r.wantStatus(s.t, http.StatusOK)

		if r.Header().Get("Content-Type") != "image/png" || r.Header().Get("X-Content-Type-Options") != "nosniff" {
			s.t.Fatalf("icon headers = %v", r.Header())
		}

		cfg, err := png.DecodeConfig(r.Body)
		if err != nil {
			s.t.Fatalf("icon is not a PNG: %v", err)
		}

		return cfg.Width
	}

	// Icons of 72 and 96 px: the smallest that fits, else the largest.
	for size, want := range map[string]int{"50": 72, "80": 96, "96": 96, "500": 96, "0": 96} {
		if got := width(size); got != want {
			s.t.Errorf("icon for size %s is %dpx, want %dpx", size, got, want)
		}
	}

	s.e.getJSON(apiBase+"/versions/"+s.patchedID+"/icon?role=monochrome").wantStatus(s.t, http.StatusNotFound)
}

func (s *scenario) scoping() {
	s.e.getJSON("/api/v1/repositories/other/apks/versions/"+s.patchedID).wantStatus(s.t, http.StatusNotFound)
	s.e.getJSON("/api/v1/repositories/plain/apks/apps").wantStatus(s.t, http.StatusNotFound)
	s.e.getJSON("/api/v1/repositories/nope/apks/apps").wantStatus(s.t, http.StatusNotFound)
	s.e.do(http.MethodDelete, "/api/v1/repositories/other/apks/versions/"+s.patchedID, nil, "").wantStatus(s.t, http.StatusNotFound)
	s.e.getJSON("/repos/other/apps/org.example.app/latest.apk").wantStatus(s.t, http.StatusNotFound)
}

func (s *scenario) badUploads() {
	e, apk50 := s.e, demoAPK(50, s.mine)

	e.upload("apps", nil, part{"file", "x.apk", []byte("not a zip")}).wantStatus(s.t, http.StatusBadRequest)
	e.upload("apps", nil, part{"file", "x.apkm", s.original}).wantStatus(s.t, http.StatusBadRequest)
	e.upload("apps", map[string]string{"origin": "bogus"}, part{"file", "x.apk", apk50}).wantStatus(s.t, http.StatusBadRequest)
	e.upload("apps", map[string]string{"tags": "bad tag"}, part{"file", "x.apk", apk50}).wantStatus(s.t, http.StatusBadRequest)
	e.upload("apps", nil).wantStatus(s.t, http.StatusBadRequest)
	e.upload("plain", nil, part{"file", "x.apk", apk50}).wantStatus(s.t, http.StatusNotFound)

	other := apktest.Builder{Package: "org.other", VersionCode: 1, MinSDK: 21, TargetSDK: 34}
	e.upload("apps", map[string]string{"origin": "patched"}, part{"file", "x.apk", apk50},
		part{"original", "y.apk", other.Build(apktest.Sign{Key: s.publisher, V2: true})}).wantStatus(s.t, http.StatusBadRequest)
}

func (s *scenario) rejectPolicy() {
	s.e.putJSON(apiBase+"/apps/org.example.app/meta", map[string]any{"signer_policy": "reject", "notes": "mine"}).wantStatus(s.t, http.StatusOK)

	stranger := apktest.NewRSAKey("Stranger")
	fields := map[string]string{"origin": "patched", "no_original": "true"}

	r := s.e.upload("apps", fields, part{"file", "s.apk", demoAPK(12, stranger)})
	r.wantStatus(s.t, http.StatusConflict)

	if !strings.Contains(r.Body.String(), stranger.SHA256()) {
		s.t.Fatalf("rejection does not name the key: %s", r.Body.String())
	}

	fields["accept_new_signer"] = "true"
	s.e.upload("apps", fields, part{"file", "s.apk", demoAPK(12, stranger)}).wantStatus(s.t, http.StatusCreated)
}

func (s *scenario) versionMeta() {
	meta := apiBase + "/versions/" + s.patchedID + "/meta"

	s.e.putJSON(meta, map[string]any{
		"notes": "n", "changelog": "c", "channel": "Beta", "pinned": true, "tags": []string{"Keep"},
	}).wantStatus(s.t, http.StatusOK)

	s.e.putJSON(meta, map[string]any{"parent_id": "not-a-uuid"}).wantStatus(s.t, http.StatusBadRequest)
	s.e.putJSON(meta, map[string]any{"source_url": "javascript:alert(1)"}).wantStatus(s.t, http.StatusBadRequest)
	s.e.putJSON(meta, map[string]any{"channel": "bad channel"}).wantStatus(s.t, http.StatusBadRequest)
}

func (s *scenario) listing() {
	// "dimost" only occurs in the Italian title, not in the default one.
	if apps := s.e.getJSON(apiBase+"/apps?q=dimost&origin=patched").data(s.t, http.StatusOK); apps["total"].(float64) != 1 {
		s.t.Fatalf("apps = %v", apps)
	}

	s.e.putJSON(apiBase+"/apps/org.example.app/meta", map[string]any{"tags": []string{"daily"}}).wantStatus(s.t, http.StatusOK)

	if d := s.e.getJSON(apiBase+"/apps?tag=daily").data(s.t, http.StatusOK); d["total"].(float64) != 1 {
		s.t.Fatalf("tag filter = %v", d)
	}

	app := s.e.getJSON(apiBase+"/apps/org.example.app").data(s.t, http.StatusOK)
	if signers, _ := app["signers"].([]any); len(signers) != 3 {
		s.t.Fatalf("signers used = %v", signers)
	}

	tags := s.e.getJSON(apiBase + "/tags").Body.String()
	for _, want := range []string{`"daily"`, `"keep"`, `"base"`} {
		if !strings.Contains(tags, want) {
			s.t.Errorf("tags %s lack %s", tags, want)
		}
	}
}

func (s *scenario) attachments() {
	base := apiBase + "/versions/" + s.patchedID + "/files"
	body, ct := multipartBody(s.t, map[string]string{"kind": "mapping", "name": "../../etc/mapping.txt"}, part{"file", "m.txt", []byte("a -> b")})
	f := s.e.do(http.MethodPost, base, body, ct).data(s.t, http.StatusCreated)

	if f["name"] != "mapping.txt" || f["kind"] != "mapping" {
		s.t.Fatalf("file = %v", f)
	}

	id, _ := f["id"].(string)

	got := s.e.getJSON(base + "/" + id)
	if got.Body.String() != "a -> b" || got.Header().Get("Content-Type") != "application/octet-stream" ||
		!strings.HasPrefix(got.Header().Get("Content-Disposition"), "attachment") {
		s.t.Fatalf("file download = %q %v", got.Body.String(), got.Header())
	}

	body, ct = multipartBody(s.t, map[string]string{"kind": "weird"}, part{"file", "m.txt", []byte("x")})
	s.e.do(http.MethodPost, base, body, ct).wantStatus(s.t, http.StatusBadRequest)

	s.e.do(http.MethodDelete, base+"/"+id, nil, "").wantStatus(s.t, http.StatusOK)
	s.e.getJSON(base+"/"+id).wantStatus(s.t, http.StatusNotFound)
}

func (s *scenario) deletion() {
	s.e.do(http.MethodDelete, apiBase+"/versions/"+s.patchedID, nil, "").wantStatus(s.t, http.StatusOK)
	s.e.getJSON(apiBase+"/versions/"+s.patchedID).wantStatus(s.t, http.StatusNotFound)
	s.e.getJSON(filePath+"10/"+s.fileName(10, s.mine)).wantStatus(s.t, http.StatusNotFound)

	sum := sha256.Sum256(s.patched)
	if rc, err := s.e.blobs.Get(context.Background(), hex.EncodeToString(sum[:])); err == nil {
		_ = rc.Close()

		s.t.Error("the deleted file is still in the blob store")
	}

	// The original is still there and still downloadable.
	s.e.getJSON(filePath+"10/"+s.fileName(10, s.publisher)).wantStatus(s.t, http.StatusOK)
}
