// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
	pkgnpm "github.com/openartifact/openartifact/pkg/npm"
)

func buildTarballBytes(t *testing.T, manifest map[string]any) []byte {
	t.Helper()

	jsonBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{Name: "package/package.json", Mode: 0o644, Size: int64(len(jsonBytes))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}

	if _, err := tw.Write(jsonBytes); err != nil {
		t.Fatalf("tar write: %v", err)
	}

	_ = tw.Close()
	_ = gw.Close()

	return buf.Bytes()
}

func buildPublishBody(t *testing.T, packageName, version string) []byte {
	t.Helper()
	tarball := buildTarballBytes(t, map[string]any{"name": packageName, "version": version})

	integrity, sha1Hex, _, _, err := pkgnpm.ComputeIntegrity(bytes.NewReader(tarball))
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}

	tarballName := packageName + "-" + version + ".tgz"
	if len(packageName) > 0 && packageName[0] == '@' {
		for i, c := range packageName {
			if c == '/' {
				tarballName = packageName[i+1:] + "-" + version + ".tgz"
				break
			}
		}
	}

	manifest := map[string]any{
		"name":    packageName,
		"version": version,
		"dist": map[string]any{
			"integrity": integrity,
			"shasum":    sha1Hex,
			"tarball":   "http://example.com/" + tarballName,
		},
	}
	manifestRaw, _ := json.Marshal(manifest)

	body := map[string]any{
		"name": packageName,
		"versions": map[string]any{
			version: json.RawMessage(manifestRaw),
		},
		"dist-tags": map[string]string{"latest": version},
		"_attachments": map[string]any{
			tarballName: map[string]any{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString(tarball),
				"length":       len(tarball),
			},
		},
	}
	raw, _ := json.Marshal(body)

	return raw
}

func setupUploadRouter(db *MockDatabase, bs *MockBlobStore) chi.Router {
	log, _ := logger.New("info", "json", "stdout")
	h := api.NewNPMUploadHandler(log, db, bs, "http://localhost:8080", nil)
	router := chi.NewRouter()
	h.Register(router)

	return router
}

func TestNPMPublish_FirstPublish_201(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	body := buildPublishBody(t, "lodash", "1.0.0")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	pkg, _ := db.GetNPMPackageByName(req.Context(), "repo-1", "lodash")
	if pkg == nil {
		t.Fatal("NPMPackage not created")
	}

	ver, _ := db.GetNPMVersion(req.Context(), pkg.ID, "1.0.0")
	if ver == nil {
		t.Fatal("NPMVersion not created")
	}

	tags, _ := db.GetNPMDistTags(req.Context(), pkg.ID)
	if tags["latest"] != "1.0.0" {
		t.Errorf("dist-tag latest = %q, want 1.0.0", tags["latest"])
	}
}

func TestNPMPublish_NewVersion_201(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	body1 := buildPublishBody(t, "lodash", "1.0.0")
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(body1))
	req1.Header.Set("Authorization", "Bearer token")

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first publish status = %d", w1.Code)
	}

	body2 := buildPublishBody(t, "lodash", "2.0.0")
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(body2))
	req2.Header.Set("Authorization", "Bearer token")

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Errorf("second publish status = %d, want 201; body: %s", w2.Code, w2.Body.String())
	}
}

func TestNPMPublish_RepublishConflict_409(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	body := buildPublishBody(t, "lodash", "1.0.0")
	for i := range 2 {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i == 1 && w.Code != http.StatusConflict {
			t.Errorf("republish status = %d, want 409; body: %s", w.Code, w.Body.String())
		}
	}
}

func TestNPMPublish_NameMismatch_400(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	body := buildPublishBody(t, "other-name", "1.0.0")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestNPMPublish_IntegrityMismatch_400(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	tarball := buildTarballBytes(t, map[string]any{"name": "lodash", "version": "1.0.0"})
	manifest := map[string]any{
		"name":    "lodash",
		"version": "1.0.0",
		"dist": map[string]any{
			"integrity": "sha512-wronghash",
			"shasum":    "wrongsha1",
		},
	}
	manifestRaw, _ := json.Marshal(manifest)
	bodyMap := map[string]any{
		"name":      "lodash",
		"versions":  map[string]any{"1.0.0": json.RawMessage(manifestRaw)},
		"dist-tags": map[string]string{"latest": "1.0.0"},
		"_attachments": map[string]any{
			"lodash-1.0.0.tgz": map[string]any{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString(tarball),
				"length":       len(tarball),
			},
		},
	}
	raw, _ := json.Marshal(bodyMap)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestNPMPublish_BodyTooLarge_413(t *testing.T) {
	t.Skip("body limit covered by http.MaxBytesReader; cannot easily test without 200MB body")
}

func TestNPMPublish_MissingAttachment_400(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	bodyMap := map[string]any{
		"name":         "lodash",
		"versions":     map[string]any{"1.0.0": json.RawMessage(`{"name":"lodash","version":"1.0.0"}`)},
		"dist-tags":    map[string]string{"latest": "1.0.0"},
		"_attachments": map[string]any{},
	}
	raw, _ := json.Marshal(bodyMap)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestNPMPublish_NoAuth_401(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupUploadRouter(db, bs)

	body := buildPublishBody(t, "lodash", "1.0.0")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/repos/myrepo/lodash", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
