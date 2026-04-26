// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api"
	"github.com/openartifact/openartifact/internal/database/models"
	fmtnpm "github.com/openartifact/openartifact/internal/format/npm"
	"github.com/openartifact/openartifact/pkg/logger"
)

func TestCreateNPMPackage(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	pkg := &models.NPMPackage{
		RepositoryID: "repo-1",
		FullName:     "lodash",
		Name:         "lodash",
	}

	created, err := db.CreateNPMPackage(ctx, pkg)
	if err != nil {
		t.Fatalf("CreateNPMPackage: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected ID to be assigned")
	}

	found, err := db.GetNPMPackageByName(ctx, "repo-1", "lodash")
	if err != nil {
		t.Fatalf("GetNPMPackageByName: %v", err)
	}

	if found == nil {
		t.Fatal("expected non-nil result")
	}

	if found.ID != created.ID {
		t.Errorf("ID mismatch: got %s, want %s", found.ID, created.ID)
	}
}

func TestGetNPMPackageByName_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	found, err := db.GetNPMPackageByName(ctx, "repo-1", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if found != nil {
		t.Fatal("expected nil result")
	}
}

func TestMarkPackumentStale(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	pkg := &models.NPMPackage{ID: "pkg-1", RepositoryID: "repo-1", FullName: "lodash"}
	db.npmPackages["pkg-1"] = pkg

	if err := db.MarkPackumentStale(ctx, "pkg-1"); err != nil {
		t.Fatalf("MarkPackumentStale: %v", err)
	}

	if !db.npmPackages["pkg-1"].PackumentStale {
		t.Error("expected PackumentStale to be true")
	}
}

func TestCreateNPMVersion(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	v := &models.NPMVersion{PackageID: "pkg-1", Version: "1.0.0", ManifestJSON: []byte(`{}`)}

	created, err := db.CreateNPMVersion(ctx, v)
	if err != nil {
		t.Fatalf("CreateNPMVersion: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected ID to be assigned")
	}

	found, err := db.GetNPMVersion(ctx, "pkg-1", "1.0.0")
	if err != nil {
		t.Fatalf("GetNPMVersion: %v", err)
	}

	if found == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestCreateNPMVersion_Duplicate(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	v := &models.NPMVersion{PackageID: "pkg-1", Version: "1.0.0"}
	if _, err := db.CreateNPMVersion(ctx, v); err != nil {
		t.Fatalf("first CreateNPMVersion: %v", err)
	}

	if _, err := db.CreateNPMVersion(ctx, &models.NPMVersion{PackageID: "pkg-1", Version: "1.0.0"}); err == nil {
		t.Error("expected error on duplicate insert")
	}
}

func TestGetNPMVersionsByPackage(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.npmVersions["pkg-1:1.0.0"] = &models.NPMVersion{ID: "v1", PackageID: "pkg-1", Version: "1.0.0"}
	db.npmVersions["pkg-1:1.1.0"] = &models.NPMVersion{ID: "v2", PackageID: "pkg-1", Version: "1.1.0"}
	db.npmVersions["pkg-2:2.0.0"] = &models.NPMVersion{ID: "v3", PackageID: "pkg-2", Version: "2.0.0"}

	vers, err := db.GetNPMVersionsByPackage(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("GetNPMVersionsByPackage: %v", err)
	}

	if len(vers) != 2 {
		t.Errorf("expected 2 versions, got %d", len(vers))
	}
}

func TestDeleteNPMVersion(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.npmVersions["pkg-1:1.0.0"] = &models.NPMVersion{ID: "v1", PackageID: "pkg-1", Version: "1.0.0"}

	if err := db.DeleteNPMVersion(ctx, "pkg-1", "1.0.0"); err != nil {
		t.Fatalf("DeleteNPMVersion: %v", err)
	}

	if found, _ := db.GetNPMVersion(ctx, "pkg-1", "1.0.0"); found != nil {
		t.Error("expected version to be deleted")
	}
}

func TestUpsertNPMDistTag_Insert(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	tag := &models.NPMDistTag{PackageID: "pkg-1", Tag: "latest", Version: "1.0.0"}
	if _, err := db.UpsertNPMDistTag(ctx, tag); err != nil {
		t.Fatalf("UpsertNPMDistTag: %v", err)
	}

	tags, err := db.GetNPMDistTags(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("GetNPMDistTags: %v", err)
	}

	if tags["latest"] != "1.0.0" {
		t.Errorf("latest = %q, want 1.0.0", tags["latest"])
	}
}

func TestUpsertNPMDistTag_Update(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	_, _ = db.UpsertNPMDistTag(ctx, &models.NPMDistTag{PackageID: "pkg-1", Tag: "latest", Version: "1.0.0"})
	_, _ = db.UpsertNPMDistTag(ctx, &models.NPMDistTag{PackageID: "pkg-1", Tag: "latest", Version: "2.0.0"})

	tags, _ := db.GetNPMDistTags(ctx, "pkg-1")
	if tags["latest"] != "2.0.0" {
		t.Errorf("latest = %q after update, want 2.0.0", tags["latest"])
	}
}

func TestGetNPMDistTags(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.npmDistTags["pkg-1"] = map[string]string{"latest": "1.0.0", "beta": "2.0.0-beta"}

	tags, err := db.GetNPMDistTags(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("GetNPMDistTags: %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestDeleteNPMDistTag(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.npmDistTags["pkg-1"] = map[string]string{"latest": "1.0.0", "beta": "2.0.0-beta"}
	if err := db.DeleteNPMDistTag(ctx, "pkg-1", "beta"); err != nil {
		t.Fatalf("DeleteNPMDistTag: %v", err)
	}

	tags, _ := db.GetNPMDistTags(ctx, "pkg-1")
	if _, has := tags["beta"]; has {
		t.Error("expected beta tag to be deleted")
	}

	if tags["latest"] != "1.0.0" {
		t.Error("expected latest tag to remain")
	}
}

func setupNPMRouter(db *MockDatabase, bs *MockBlobStore) chi.Router {
	log, _ := logger.New("info", "json", "stdout")
	h := fmtnpm.NewHTTPHandler(log, db, bs, "http://localhost:8080")
	router := chi.NewRouter()
	h.RegisterRoutes(router, "/repos/{name}")

	return router
}

func TestNPMHTTPHandler_ServePackument_200(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()

	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}
	db.npmPackages["pkg-1"] = &models.NPMPackage{
		ID: "pkg-1", RepositoryID: "repo-1", FullName: "lodash", Name: "lodash",
		UpdatedAt: time.Now(),
	}
	raw, _ := json.Marshal(map[string]any{"name": "lodash", "version": "1.0.0"})
	db.npmVersions["pkg-1:1.0.0"] = &models.NPMVersion{
		ID: "v1", PackageID: "pkg-1", Version: "1.0.0",
		ManifestJSON: raw, TarballFilename: "lodash-1.0.0.tgz",
	}
	db.npmDistTags["pkg-1"] = map[string]string{"latest": "1.0.0"}

	router := setupNPMRouter(db, bs)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/repos/myrepo/lodash", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	body := w.Body.Bytes()
	if !bytes.Contains(body, []byte("lodash")) {
		t.Errorf("body does not contain 'lodash': %s", body)
	}

	if !bytes.Contains(body, []byte("dist-tags")) {
		t.Errorf("body does not contain 'dist-tags': %s", body)
	}
}

func TestNPMHTTPHandler_ServePackument_404(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	router := setupNPMRouter(db, bs)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/repos/myrepo/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestNPMHTTPHandler_ScopedName(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()

	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}
	db.npmPackages["pkg-1"] = &models.NPMPackage{
		ID: "pkg-1", RepositoryID: "repo-1", FullName: "@scope/name", Name: "name",
		UpdatedAt: time.Now(),
	}
	raw, _ := json.Marshal(map[string]any{"name": "@scope/name", "version": "1.0.0"})
	db.npmVersions["pkg-1:1.0.0"] = &models.NPMVersion{
		ID: "v1", PackageID: "pkg-1", Version: "1.0.0",
		ManifestJSON: raw, TarballFilename: "name-1.0.0.tgz",
	}
	db.npmDistTags["pkg-1"] = map[string]string{"latest": "1.0.0"}

	router := setupNPMRouter(db, bs)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/repos/myrepo/@scope%2Fname", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestNPMHTTPHandler_GetDistTags_200(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()

	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}
	db.npmPackages["pkg-1"] = &models.NPMPackage{
		ID: "pkg-1", RepositoryID: "repo-1", FullName: "lodash",
	}
	db.npmDistTags["pkg-1"] = map[string]string{"latest": "1.0.0", "beta": "2.0.0-beta"}

	router := setupNPMRouter(db, bs)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/repos/myrepo/-/package/lodash/dist-tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var tags map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &tags); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if tags["latest"] != "1.0.0" {
		t.Errorf("latest = %q, want 1.0.0", tags["latest"])
	}
}

func TestNPMHTTPHandler_PutDistTag_RequiresAuth(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()
	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}

	log, _ := logger.New("info", "json", "stdout")
	h := api.NewNPMUploadHandler(log, db, bs, "http://localhost:8080", nil)
	router := chi.NewRouter()
	h.Register(router)

	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/repos/myrepo/-/package/lodash/dist-tags/latest", bytes.NewBufferString(`"1.0.0"`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestNPMHTTPHandler_PutDistTag_200(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()

	db.repositories["repo-1"] = &models.Repository{ID: "repo-1", Name: "myrepo"}
	db.npmPackages["pkg-1"] = &models.NPMPackage{
		ID: "pkg-1", RepositoryID: "repo-1", FullName: "lodash",
	}
	db.npmVersions["pkg-1:1.0.0"] = &models.NPMVersion{
		ID: "v1", PackageID: "pkg-1", Version: "1.0.0", ManifestJSON: []byte(`{}`),
	}

	log, _ := logger.New("info", "json", "stdout")
	h := api.NewNPMUploadHandler(log, db, bs, "http://localhost:8080", nil)
	router := chi.NewRouter()
	h.Register(router)

	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/repos/myrepo/-/package/lodash/dist-tags/latest", bytes.NewBufferString(`"1.0.0"`))
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}
