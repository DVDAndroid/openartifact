// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api"
	"github.com/openartifact/openartifact/internal/database/models"
)

// systemOrgID is the organization seeded by the initial migration.
const systemOrgID = "00000000-0000-0000-0000-000000000001"

func buildUploadRequest(t *testing.T, rpmData []byte, repoName string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "test.rpm")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(rpmData)); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	w.Close()
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/repositories/"+repoName+"/packages", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func createTestRepo(t *testing.T, ctx context.Context, name string) *models.Repository {
	t.Helper()
	orgID := systemOrgID
	repo := &models.Repository{
		Name:        name,
		Format:      "rpm",
		Description: "integration test repo",
		ProjectID:   &orgID,
	}
	if err := harness.Database.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}
	rpmRepo := &models.RPMRepo{
		RepositoryID:  repo.ID,
		BaseArch:      "x86_64",
		MetadataStale: false,
	}
	if err := harness.Database.CreateRPMRepo(ctx, rpmRepo); err != nil {
		t.Fatalf("CreateRPMRepo: %v", err)
	}
	return repo
}

func buildUploadRouter(t *testing.T) chi.Router {
	t.Helper()
	h := api.NewRPMUploadHandler(harness.Log, harness.Database, harness.BlobStore, nil)
	router := chi.NewRouter()
	h.Register(router)
	return router
}

func uploadRPM(t *testing.T, router chi.Router, rpmData []byte, repoName string) *httptest.ResponseRecorder {
	t.Helper()
	req := buildUploadRequest(t, rpmData, repoName)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestRPMUpload_Success(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "test-upload-repo")
	rpmData := CreateTestRPM("mypkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp)
	}
}

func TestRPMUpload_ComponentRow(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "test-component-repo")
	rpmData := CreateTestRPM("mypkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	filename := "mypkg-1.0.0-1.el9.x86_64.rpm"
	comp, err := harness.Database.GetComponentByFilename(ctx, repo.ID, filename)
	if err != nil {
		t.Fatalf("GetComponentByFilename: %v", err)
	}
	if comp == nil {
		t.Fatal("expected component row to exist")
	}
	if comp.Format != "rpm" {
		t.Errorf("Format = %q, want %q", comp.Format, "rpm")
	}
	if comp.Name != "mypkg" {
		t.Errorf("Name = %q, want %q", comp.Name, "mypkg")
	}
	if comp.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", comp.Version, "1.0.0")
	}
	if comp.Release != "1.el9" {
		t.Errorf("Release = %q, want %q", comp.Release, "1.el9")
	}
	if comp.Arch != "x86_64" {
		t.Errorf("Arch = %q, want %q", comp.Arch, "x86_64")
	}
	if comp.SHA256 == "" {
		t.Error("expected SHA256 to be populated")
	}
	if want := fmt.Sprintf("%x", sha256.Sum256(rpmData)); comp.SHA256 != want {
		t.Errorf("SHA256 = %q, want %q", comp.SHA256, want)
	}
}

func TestRPMUpload_RPMColumns(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "test-rpm-columns-repo")
	rpmData := CreateTestRPM("colpkg", "2.1.0", "3.el9", "aarch64")
	router := buildUploadRouter(t)

	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	filename := "colpkg-2.1.0-3.el9.aarch64.rpm"
	comp, err := harness.Database.GetComponentByFilename(ctx, repo.ID, filename)
	if err != nil {
		t.Fatalf("GetComponentByFilename: %v", err)
	}
	if comp == nil {
		t.Fatal("expected component row")
	}
	if comp.Summary == "" {
		t.Error("expected Summary to be populated")
	}
	if comp.Description == "" {
		t.Error("expected Description to be populated")
	}
	if comp.License == "" {
		t.Error("expected License to be populated")
	}
}

func TestRPMUpload_BlobExists(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "test-blob-repo")
	rpmData := CreateTestRPM("blobpkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)

	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	filename := "blobpkg-1.0.0-1.el9.x86_64.rpm"
	comp, err := harness.Database.GetComponentByFilename(ctx, repo.ID, filename)
	if err != nil {
		t.Fatalf("GetComponentByFilename: %v", err)
	}
	if comp == nil {
		t.Fatal("expected component row")
	}

	exists, err := harness.BlobStore.Exists(ctx, comp.SHA256)
	if err != nil {
		t.Fatalf("BlobStore.Exists: %v", err)
	}
	if !exists {
		t.Error("expected blob to exist in store")
	}

	reader, err := harness.BlobStore.Get(ctx, comp.SHA256)
	if err != nil {
		t.Fatalf("BlobStore.Get: %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll blob: %v", err)
	}

	gotSHA := fmt.Sprintf("%x", sha256.Sum256(content))
	if gotSHA != comp.SHA256 {
		t.Errorf("blob SHA256 = %q, want %q", gotSHA, comp.SHA256)
	}
}
