// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openartifact/openartifact/internal/worker"
)

func runMetadataRegen(t *testing.T, repoID string) {
	t.Helper()
	w := worker.NewRPMetadataWorker(harness.Database, harness.BlobStore, nil, harness.Log, nil)
	w.NotifyStale(repoID)
	time.Sleep(700 * time.Millisecond)
	w.Stop()
}

func TestRPMMetadataRegeneration(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "meta-regen-repo")
	rpmData := CreateTestRPM("metapkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)
	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	if err := harness.Database.MarkRepoStale(ctx, repo.ID); err != nil {
		t.Fatalf("MarkRepoStale: %v", err)
	}

	runMetadataRegen(t, repo.ID)

	metas, err := harness.Database.GetRepoMetadataByRepo(ctx, repo.ID)
	if err != nil {
		t.Fatalf("GetRepoMetadataByRepo: %v", err)
	}
	if len(metas) == 0 {
		t.Fatal("expected repo_metadata entries after regeneration")
	}

	paths := make(map[string]bool)
	for _, m := range metas {
		paths[m.Path] = true
	}
	for _, want := range []string{
		"repodata/repomd.xml",
	} {
		if !paths[want] {
			t.Errorf("missing expected path %q in repo_metadata", want)
		}
	}

	hasPrimary := false
	hasFilelists := false
	hasOther := false
	for p := range paths {
		if strings.Contains(p, "primary.xml.gz") {
			hasPrimary = true
		}
		if strings.Contains(p, "filelists.xml.gz") {
			hasFilelists = true
		}
		if strings.Contains(p, "other.xml.gz") {
			hasOther = true
		}
	}
	if !hasPrimary {
		t.Error("missing primary.xml.gz in repo_metadata")
	}
	if !hasFilelists {
		t.Error("missing filelists.xml.gz in repo_metadata")
	}
	if !hasOther {
		t.Error("missing other.xml.gz in repo_metadata")
	}
}

func TestRPMMetadataRegeneration_StaleCleared(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "meta-stale-repo")
	rpmData := CreateTestRPM("stalepkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)
	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	if err := harness.Database.MarkRepoStale(ctx, repo.ID); err != nil {
		t.Fatalf("MarkRepoStale: %v", err)
	}

	runMetadataRegen(t, repo.ID)

	rpmRepo, err := harness.Database.GetRPMRepo(ctx, repo.ID)
	if err != nil {
		t.Fatalf("GetRPMRepo: %v", err)
	}
	if rpmRepo == nil {
		t.Fatal("expected rpm_repos row")
	}
	if rpmRepo.MetadataStale {
		t.Error("expected metadata_stale = false after regeneration")
	}
}

func TestRPMMetadataRegeneration_BlobsExist(t *testing.T) {
	harness.Reset(t)
	ctx := context.Background()

	repo := createTestRepo(t, ctx, "meta-blobs-repo")
	rpmData := CreateTestRPM("blobmetapkg", "1.0.0", "1.el9", "x86_64")
	router := buildUploadRouter(t)
	w := uploadRPM(t, router, rpmData, repo.Name)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	if err := harness.Database.MarkRepoStale(ctx, repo.ID); err != nil {
		t.Fatalf("MarkRepoStale: %v", err)
	}

	runMetadataRegen(t, repo.ID)

	metas, err := harness.Database.GetRepoMetadataByRepo(ctx, repo.ID)
	if err != nil {
		t.Fatalf("GetRepoMetadataByRepo: %v", err)
	}

	for _, m := range metas {
		if len(m.BlobSHA256) != 64 {
			t.Errorf("path %q: blob_sha256 length = %d, want 64", m.Path, len(m.BlobSHA256))
		}
		exists, err := harness.BlobStore.Exists(ctx, m.BlobSHA256)
		if err != nil {
			t.Errorf("path %q: BlobStore.Exists error: %v", m.Path, err)
			continue
		}
		if !exists {
			t.Errorf("path %q: blob %q not found in store", m.Path, m.BlobSHA256)
		}
	}
}
