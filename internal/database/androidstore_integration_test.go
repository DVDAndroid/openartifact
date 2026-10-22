// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package database

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openartifact/openartifact/internal/database/models"
)

func androidTestRepo(t *testing.T, pg *PostgresDB, name string) string {
	t.Helper()

	org := systemOrgID

	repo := &models.Repository{Name: name, Format: "android", RepoType: "hosted", OrgID: &org}
	if err := pg.CreateRepository(context.Background(), repo); err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}

	return repo.ID
}

func androidTestBlob(t *testing.T, pg *PostgresDB, seed string, size int64) (string, string) {
	t.Helper()

	sha := strings.Repeat(seed, 64)[:64]

	id, err := pg.UpsertBlob(context.Background(), sha, sha, size)
	if err != nil {
		t.Fatalf("UpsertBlob: %v", err)
	}

	return id, sha
}

func androidTestVersion(t *testing.T, pg *PostgresDB, repoID string, app *models.AndroidApp, code int64, signer, origin, seed string) *models.AndroidVersion {
	t.Helper()

	blobID, sha := androidTestBlob(t, pg, seed, 100)
	v := &models.AndroidVersion{
		RepositoryID: repoID, AppID: app.ID, PackageName: app.PackageName, VersionCode: code, VersionName: "v",
		FileKind: "apk", BlobID: blobID, SHA256: sha, SizeBytes: 100, Filename: "a-" + seed + ".apk",
		SignerSHA256: signer, VariantKey: signer[:63] + seed, Label: "Demo", MinSDK: 21, Origin: origin,
		Labels: json.RawMessage(`{"":"Demo","it":"Dimostrazione"}`), ABIs: []string{"arm64-v8a"},
	}

	if err := pg.CreateAndroidVersion(context.Background(), v, nil); err != nil {
		t.Fatalf("CreateAndroidVersion(%d, %s): %v", code, origin, err)
	}

	return v
}

func mustApp(t *testing.T, pg *PostgresDB, repoID, pkg string) *models.AndroidApp {
	t.Helper()

	a, err := pg.UpsertAndroidApp(context.Background(), repoID, pkg)
	if err != nil {
		t.Fatal(err)
	}

	return a
}

// A blob only an Android row points to is referenced, so the orphan collector
// and SafeDeleteBlob leave it alone.
func TestAndroidBlobReferences(t *testing.T) {
	pg := newQuotaTestDB(t, 4)
	ctx := context.Background()
	repoID := androidTestRepo(t, pg, "apps")
	app := mustApp(t, pg, repoID, "org.example")

	apk, apkSHA := androidTestBlob(t, pg, "1", 100)
	icon, iconSHA := androidTestBlob(t, pg, "2", 10)
	file, fileSHA := androidTestBlob(t, pg, "3", 20)
	_, orphanSHA := androidTestBlob(t, pg, "4", 5)

	v := &models.AndroidVersion{
		RepositoryID: repoID, AppID: app.ID, PackageName: app.PackageName, VersionCode: 1, FileKind: "apk",
		BlobID: apk, SHA256: apkSHA, SizeBytes: 100, Filename: "a.apk", VariantKey: "k", Origin: "built",
	}
	icons := []models.AndroidIcon{{Role: "app", Mime: "image/png", BlobID: icon, SHA256: iconSHA, SizeBytes: 10}}

	if err := pg.CreateAndroidVersion(ctx, v, icons); err != nil {
		t.Fatal(err)
	}

	f := &models.AndroidFile{VersionID: v.ID, Kind: "other", Name: "n", BlobID: file, SHA256: fileSHA, SizeBytes: 20}
	if err := pg.CreateAndroidFile(ctx, repoID, f); err != nil {
		t.Fatal(err)
	}

	for name, sha := range map[string]string{"apk": apkSHA, "icon": iconSHA, "file": fileSHA} {
		if n, err := pg.BlobReferenceCount(ctx, sha); err != nil || n != 1 {
			t.Errorf("%s blob: reference count %d, %v; want 1", name, n, err)
		}
	}

	if n, _ := pg.BlobReferenceCount(ctx, orphanSHA); n != 0 {
		t.Errorf("unreferenced blob counted %d references", n)
	}

	if _, err := pg.DeleteAndroidVersion(ctx, repoID, v.ID); err != nil {
		t.Fatal(err)
	}

	if n, _ := pg.BlobReferenceCount(ctx, apkSHA); n != 0 {
		t.Errorf("blob still referenced after the version was deleted: %d", n)
	}
}
