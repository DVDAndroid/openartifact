// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/database/models"
	fmtmaven "github.com/openartifact/openartifact/internal/format/maven"
	"github.com/openartifact/openartifact/pkg/logger"
)

func TestCreateMavenArtifact(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	a := &models.MavenArtifact{
		RepositoryID:  "repo-1",
		GroupID:       "com.example",
		ArtifactCoord: "myapp",
		Version:       "1.0.0",
		Extension:     "jar",
		RepoPath:      "com/example/myapp/1.0.0/myapp-1.0.0.jar",
		SHA256:        "abc123",
	}
	if err := db.CreateMavenArtifact(ctx, a); err != nil {
		t.Fatalf("CreateMavenArtifact error: %v", err)
	}

	if a.ID == "" {
		t.Fatal("expected ID to be set after create")
	}

	if _, ok := db.mavenArtifacts[a.ID]; !ok {
		t.Fatal("artifact not stored in mock")
	}
}

func TestGetMavenArtifactByPath_Found(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	a := &models.MavenArtifact{
		ID:            "art-1",
		RepositoryID:  "repo-1",
		GroupID:       "com.example",
		ArtifactCoord: "myapp",
		Version:       "1.0.0",
		Extension:     "jar",
		RepoPath:      "com/example/myapp/1.0.0/myapp-1.0.0.jar",
		SHA256:        "abc123",
	}
	db.mavenArtifacts["art-1"] = a

	result, err := db.GetMavenArtifactByPath(ctx, "repo-1", "com/example/myapp/1.0.0/myapp-1.0.0.jar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.ID != "art-1" {
		t.Errorf("ID = %q, want %q", result.ID, "art-1")
	}
}

func TestGetMavenArtifactByPath_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	result, err := db.GetMavenArtifactByPath(ctx, "repo-1", "nonexistent/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Fatal("expected nil result")
	}
}

func TestGetMavenArtifactsByGA(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.mavenArtifacts["art-1"] = &models.MavenArtifact{
		ID: "art-1", RepositoryID: "repo-1", GroupID: "com.example", ArtifactCoord: "myapp", Version: "1.0.0",
	}
	db.mavenArtifacts["art-2"] = &models.MavenArtifact{
		ID: "art-2", RepositoryID: "repo-1", GroupID: "com.example", ArtifactCoord: "myapp", Version: "1.1.0",
	}
	db.mavenArtifacts["art-3"] = &models.MavenArtifact{
		ID: "art-3", RepositoryID: "repo-1", GroupID: "com.other", ArtifactCoord: "myapp", Version: "1.0.0",
	}

	results, err := db.GetMavenArtifactsByGA(ctx, "repo-1", "com.example", "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestGetMavenArtifactsByGAV(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.mavenArtifacts["art-1"] = &models.MavenArtifact{
		ID: "art-1", RepositoryID: "repo-1", GroupID: "com.example", ArtifactCoord: "myapp", Version: "1.0.0",
	}
	db.mavenArtifacts["art-2"] = &models.MavenArtifact{
		ID: "art-2", RepositoryID: "repo-1", GroupID: "com.example", ArtifactCoord: "myapp", Version: "1.1.0",
	}

	results, err := db.GetMavenArtifactsByGAV(ctx, "repo-1", "com.example", "myapp", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.mavenArtifacts["art-1"] = &models.MavenArtifact{
		ID: "art-1", RepositoryID: "repo-1", GroupID: "com.example", ArtifactCoord: "myapp",
		Version: "1.0.0-SNAPSHOT", IsSnapshot: true, SnapshotTimestamp: "20260426.110000",
	}
	db.mavenArtifacts["art-2"] = &models.MavenArtifact{
		ID: "art-2", RepositoryID: "repo-1", GroupID: "com.example", ArtifactCoord: "myapp",
		Version: "1.0.0-SNAPSHOT", IsSnapshot: true, SnapshotTimestamp: "20260426.120000",
	}

	result, err := db.GetLatestSnapshot(ctx, "repo-1", "com.example", "myapp", "1.0.0-SNAPSHOT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.ID != "art-2" {
		t.Errorf("expected latest art-2, got %s", result.ID)
	}
}

func TestDeleteMavenArtifact(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()

	db.mavenArtifacts["art-1"] = &models.MavenArtifact{ID: "art-1"}

	if err := db.DeleteMavenArtifact(ctx, "art-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := db.mavenArtifacts["art-1"]; ok {
		t.Fatal("artifact should be deleted")
	}
}

func TestMavenHTTP_ServeArtifact_OK(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()
	bs := newMockBlobStore()

	repo := &models.Repository{ID: "repo-1", Name: "maven-repo"}
	db.repositories["repo-1"] = repo

	content := []byte("fake-jar-content")
	blobID, _ := bs.Put(ctx, bytes.NewReader(content), nil)

	art := &models.MavenArtifact{
		ID:            "art-1",
		RepositoryID:  "repo-1",
		BlobID:        blobID,
		GroupID:       "com.example",
		ArtifactCoord: "myapp",
		Version:       "1.0.0",
		Extension:     "jar",
		RepoPath:      "com/example/myapp/1.0.0/myapp-1.0.0.jar",
		SHA256:        blobID,
	}
	db.mavenArtifacts["art-1"] = art

	log, _ := logger.New("info", "json", "stdout")
	handler := fmtmaven.NewHTTPHandler(log, db, bs)

	router := chi.NewRouter()
	handler.RegisterRoutes(router, "/repos/{name}")

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/repos/maven-repo/com/example/myapp/1.0.0/myapp-1.0.0.jar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), content) {
		t.Errorf("body mismatch: got %q, want %q", w.Body.Bytes(), content)
	}
}

func TestMavenHTTP_ServeArtifact_NotFound(t *testing.T) {
	db := newMockDatabase()
	bs := newMockBlobStore()

	repo := &models.Repository{ID: "repo-1", Name: "maven-repo"}
	db.repositories["repo-1"] = repo

	log, _ := logger.New("info", "json", "stdout")
	handler := fmtmaven.NewHTTPHandler(log, db, bs)

	router := chi.NewRouter()
	handler.RegisterRoutes(router, "/repos/{name}")

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/repos/maven-repo/com/example/myapp/1.0.0/myapp-1.0.0.jar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		body, _ := io.ReadAll(w.Body)
		t.Errorf("expected 404, got %d body=%q", w.Code, body)
	}
}
