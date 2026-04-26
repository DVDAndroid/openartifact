// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/logger"
)

func buildRPMUploadRequest(t *testing.T, rpmData []byte, repoName string) *http.Request {
	t.Helper()

	var body bytes.Buffer

	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("file", "test.rpm")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}

	if _, err := io.Copy(fw, bytes.NewReader(rpmData)); err != nil {
		t.Fatalf("Copy to form file: %v", err)
	}

	_ = w.Close()
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/repositories/"+repoName+"/packages", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	return req
}

func TestArtifactExists_NotFound(t *testing.T) {
	db := newMockDatabase()
	ctx := context.Background()

	exists, err := db.ArtifactExists(ctx, "deadbeef", "repo-1")
	if err != nil {
		t.Fatalf("ArtifactExists: %v", err)
	}

	if exists {
		t.Error("expected false for non-existent sha256+repo")
	}
}

func TestArtifactExists_Found(t *testing.T) {
	db := newMockDatabase()
	ctx := context.Background()

	comp := &models.Component{
		ID:           "comp-1",
		RepositoryID: "repo-1",
		ArtifactID:   "blob-1",
		SHA256:       "abc123",
		Format:       "rpm",
		Name:         "testpkg",
		Version:      "1.0",
	}
	db.components["comp-1"] = comp

	exists, err := db.ArtifactExists(ctx, "abc123", "repo-1")
	if err != nil {
		t.Fatalf("ArtifactExists: %v", err)
	}

	if !exists {
		t.Error("expected true for existing sha256+repo")
	}

	exists, err = db.ArtifactExists(ctx, "abc123", "repo-2")
	if err != nil {
		t.Fatalf("ArtifactExists different repo: %v", err)
	}

	if exists {
		t.Error("expected false for same sha256 but different repo")
	}
}

func TestExistsBySHA256_MockBlobStore_NotFound(t *testing.T) {
	store := newMockBlobStore()
	ctx := context.Background()

	blobID, exists, err := store.ExistsBySHA256(ctx, "nothash")
	if err != nil {
		t.Fatalf("ExistsBySHA256: %v", err)
	}

	if exists || blobID != "" {
		t.Errorf("expected not found, got exists=%v blobID=%q", exists, blobID)
	}
}

func TestExistsBySHA256_MockBlobStore_Found(t *testing.T) {
	store := newMockBlobStore()
	ctx := context.Background()
	sha := "abc123deadbeef"
	store.blobs["blob-x"] = []byte("data")
	store.sha256Map[sha] = "blob-x"

	blobID, exists, err := store.ExistsBySHA256(ctx, sha)
	if err != nil {
		t.Fatalf("ExistsBySHA256: %v", err)
	}

	if !exists {
		t.Error("expected exists=true")
	}

	if blobID != "blob-x" {
		t.Errorf("expected blobID 'blob-x', got %q", blobID)
	}
}

func TestExistsBySHA256_FilesystemStore_NotFound(t *testing.T) {
	dir := t.TempDir()

	store, err := storage.NewFilesystemStore(dir)
	if err != nil {
		t.Fatalf("NewFilesystemStore: %v", err)
	}

	blobID, exists, err := store.ExistsBySHA256(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ExistsBySHA256: %v", err)
	}

	if exists || blobID != "" {
		t.Errorf("expected not found, got exists=%v blobID=%q", exists, blobID)
	}
}

func TestExistsBySHA256_FilesystemStore_Found(t *testing.T) {
	dir := t.TempDir()

	store, err := storage.NewFilesystemStore(dir)
	if err != nil {
		t.Fatalf("NewFilesystemStore: %v", err)
	}

	ctx := context.Background()
	data := []byte("hello world content")

	blobID, err := store.Put(ctx, bytes.NewReader(data), map[string]string{})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if blobID == "" {
		t.Fatal("expected non-empty blobID from Put")
	}

	meta, err := store.GetMetadata(ctx, blobID)
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}

	actualSHA := meta["_sha256"]
	if actualSHA == "" {
		t.Fatal("expected _sha256 in metadata")
	}

	foundID, exists, err := store.ExistsBySHA256(ctx, actualSHA)
	if err != nil {
		t.Fatalf("ExistsBySHA256: %v", err)
	}

	if !exists {
		t.Error("expected exists=true after Put")
	}

	if foundID == "" {
		t.Error("expected non-empty blobID from ExistsBySHA256")
	}
}

func buildRPMRouter(t *testing.T, db *MockDatabase, blobStore *MockBlobStore, repoName string) chi.Router {
	t.Helper()
	log := newHandlerLogger(t)
	h := api.NewRPMUploadHandler(log, db, blobStore, nil)
	router := chi.NewRouter()
	h.Register(router)

	return router
}

func seedTestRepo(db *MockDatabase, id, name string) {
	db.repositories[id] = &models.Repository{
		ID:     id,
		Name:   name,
		Format: "rpm",
	}
}

func TestDedup_SamePackageTwice_SecondGets409(t *testing.T) {
	db := newMockDatabase()
	blobStore := newMockBlobStore()
	repoName := "testrepo"
	repoID := "repo-dedup-1"
	seedTestRepo(db, repoID, repoName)

	router := buildRPMRouter(t, db, blobStore, repoName)
	rpmData := buildMinimalRPM()

	req1 := buildRPMUploadRequest(t, rpmData, repoName)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	req2 := buildRPMUploadRequest(t, rpmData, repoName)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("second upload: expected 409, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal 409 response: %v", err)
	}

	if data, ok := resp["data"].(map[string]any); !ok || data["duplicate"] != true {
		t.Errorf("expected duplicate=true in response data, got %v", resp)
	}
}

func TestDedup_SamePackageDifferentRepo_Both200(t *testing.T) {
	db := newMockDatabase()
	blobStore := newMockBlobStore()

	seedTestRepo(db, "repo-a-id", "repo-a")
	seedTestRepo(db, "repo-b-id", "repo-b")

	rpmData := buildMinimalRPM()

	logA, _ := logger.New("error", "json", "stderr")
	hA := api.NewRPMUploadHandler(logA, db, blobStore, nil)
	routerA := chi.NewRouter()
	hA.Register(routerA)

	logB, _ := logger.New("error", "json", "stderr")
	hB := api.NewRPMUploadHandler(logB, db, blobStore, nil)
	routerB := chi.NewRouter()
	hB.Register(routerB)

	reqA := buildRPMUploadRequest(t, rpmData, "repo-a")
	wA := httptest.NewRecorder()
	routerA.ServeHTTP(wA, reqA)

	if wA.Code != http.StatusOK {
		t.Fatalf("repo-a upload: expected 200, got %d: %s", wA.Code, wA.Body.String())
	}

	reqB := buildRPMUploadRequest(t, rpmData, "repo-b")
	wB := httptest.NewRecorder()
	routerB.ServeHTTP(wB, reqB)

	if wB.Code != http.StatusOK {
		t.Fatalf("repo-b upload: expected 200, got %d: %s", wB.Code, wB.Body.String())
	}
}

func TestDedup_ConcurrentSamePackage_ExactlyOneSucceeds(t *testing.T) {
	db := newMockDatabase()
	blobStore := newMockBlobStore()
	repoName := "concurrent-repo"
	repoID := "repo-concurrent-1"
	seedTestRepo(db, repoID, repoName)

	log, _ := logger.New("error", "json", "stderr")
	h := api.NewRPMUploadHandler(log, db, blobStore, nil)
	router := chi.NewRouter()
	h.Register(router)

	rpmData := buildMinimalRPM()
	n := 5

	codes := make([]int, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := buildRPMUploadRequest(t, rpmData, repoName)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			codes[idx] = w.Code
		}(i)
	}

	wg.Wait()

	ok := 0
	conflict := 0

	for _, code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if ok != 1 {
		t.Errorf("expected exactly 1 success, got %d (codes: %v)", ok, codes)
	}

	if conflict != n-1 {
		t.Errorf("expected %d conflicts, got %d (codes: %v)", n-1, conflict, codes)
	}
}
