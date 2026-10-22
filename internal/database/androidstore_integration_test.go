// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package database

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
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

func androidTestBlob(t *testing.T, pg *PostgresDB, seed string, size int64) (blobID, sha string) {
	t.Helper()

	sha = strings.Repeat(seed, 64)[:64]

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
		FileKind: "apk", BlobID: blobID, SHA256: sha, SizeBytes: 100, Filename: "a.apk",
		SignerSHA256: signer, VariantKey: signer[:63] + seed, Label: "Demo", MinSDK: 21, Origin: origin,
		Labels: json.RawMessage(`{"":"Demo","it":"Dimostrazione"}`), ABIs: []string{"arm64-v8a"},
	}

	if err := pg.CreateAndroidVersion(context.Background(), v, nil); err != nil {
		t.Fatalf("CreateAndroidVersion(%d, %s): %v", code, origin, err)
	}

	return v
}

func TestAndroidStore(t *testing.T) {
	pg := newQuotaTestDB(t, 4)
	ctx := context.Background()
	repoID := androidTestRepo(t, pg, "apps")
	otherRepo := androidTestRepo(t, pg, "other")

	app, err := pg.UpsertAndroidApp(ctx, repoID, "org.example")
	if err != nil {
		t.Fatal(err)
	}

	again, err := pg.UpsertAndroidApp(ctx, repoID, "org.example")
	if err != nil || again.ID != app.ID {
		t.Fatalf("upsert is not idempotent: %v %v", again, err)
	}

	orig := androidTestVersion(t, pg, repoID, app, 10, strings.Repeat("a", 64), models.AndroidOriginOriginal, "1")
	patched := androidTestVersion(t, pg, repoID, app, 10, strings.Repeat("b", 64), models.AndroidOriginPatched, "2")
	newest := androidTestVersion(t, pg, repoID, app, 11, strings.Repeat("b", 64), models.AndroidOriginPatched, "3")

	t.Run("same variant is a duplicate, another signer is not", func(t *testing.T) {
		dup := *orig
		dup.ID = ""

		if err := pg.CreateAndroidVersion(ctx, &dup, nil); !errors.Is(err, ErrDuplicate) {
			t.Fatalf("err = %v, want ErrDuplicate", err)
		}
	})

	t.Run("versions are scoped to their repository", func(t *testing.T) {
		got, err := pg.GetAndroidVersion(ctx, otherRepo, orig.ID, true)
		if err != nil || got != nil {
			t.Fatalf("version leaked across repositories: %v %v", got, err)
		}

		got, err = pg.GetAndroidVersion(ctx, repoID, orig.ID, true)
		if err != nil || got == nil || got.ID != orig.ID {
			t.Fatalf("GetAndroidVersion = %v, %v", got, err)
		}

		if string(got.Entries) != "[]" || string(got.Manifest) != "{}" {
			t.Fatalf("detail columns = %s / %s", got.Entries, got.Manifest)
		}
	})

	t.Run("list apps aggregates versions", func(t *testing.T) {
		apps, total, err := pg.ListAndroidApps(ctx, repoID, models.AndroidAppFilter{})
		if err != nil || total != 1 || len(apps) != 1 {
			t.Fatalf("ListAndroidApps = %d apps, total %d, %v", len(apps), total, err)
		}

		a := apps[0]
		if a.VersionCount != 3 || a.LatestID != newest.ID || a.LatestCode != 11 || a.TotalSize != 300 {
			t.Fatalf("summary = %+v", a)
		}

		if !slices.Equal(a.Origins, []string{"original", "patched"}) || len(a.Signers) != 2 {
			t.Fatalf("origins %v signers %v", a.Origins, a.Signers)
		}

		for _, f := range []models.AndroidAppFilter{
			{Origin: "original"}, {Signer: strings.Repeat("a", 64)}, {Query: "org.exa"}, {Query: "demo"},
		} {
			if _, n, err := pg.ListAndroidApps(ctx, repoID, f); err != nil || n != 1 {
				t.Fatalf("filter %+v: total %d, %v", f, n, err)
			}
		}

		for _, f := range []models.AndroidAppFilter{{Origin: "built"}, {Query: "100%"}, {Tags: []string{"nope"}}} {
			if _, n, err := pg.ListAndroidApps(ctx, repoID, f); err != nil || n != 0 {
				t.Fatalf("filter %+v matched: total %d, %v", f, n, err)
			}
		}

		if _, n, err := pg.ListAndroidApps(ctx, otherRepo, models.AndroidAppFilter{}); err != nil || n != 0 {
			t.Fatalf("other repository sees %d apps (%v)", n, err)
		}
	})

	t.Run("latest honours sdk, abi and signer", func(t *testing.T) {
		cases := []struct {
			name string
			f    AndroidLatestFilter
			want string
		}{
			{"any", AndroidLatestFilter{}, newest.ID},
			{"signer", AndroidLatestFilter{Signer: strings.Repeat("a", 64)}, orig.ID},
			{"abi match", AndroidLatestFilter{ABI: "arm64-v8a"}, newest.ID},
			{"abi miss", AndroidLatestFilter{ABI: "x86"}, ""},
			{"sdk too old", AndroidLatestFilter{SDK: 19}, ""},
			{"origin", AndroidLatestFilter{Origin: "original"}, orig.ID},
		}

		for _, c := range cases {
			got, err := pg.LatestAndroidVersion(ctx, repoID, "org.example", c.f)
			if err != nil {
				t.Fatal(err)
			}

			id := ""
			if got != nil {
				id = got.ID
			}

			if id != c.want {
				t.Errorf("%s: got %q, want %q", c.name, id, c.want)
			}
		}
	})

	t.Run("download resolves by file name among variants", func(t *testing.T) {
		got, err := pg.GetAndroidDownload(ctx, repoID, "org.example", 10, "a.apk")
		if err != nil || got == nil || got.VersionCode != 10 {
			t.Fatalf("GetAndroidDownload = %v, %v", got, err)
		}

		if got, _ := pg.GetAndroidDownload(ctx, repoID, "org.example", 99, "a.apk"); got != nil {
			t.Fatal("unknown versionCode resolved")
		}
	})

	t.Run("meta updates tags, parent and notes", func(t *testing.T) {
		notes, tags := "patched with x", []string{"keep", "device:pixel"}

		got, err := pg.UpdateAndroidVersionMeta(ctx, repoID, patched.ID, models.AndroidVersionMeta{
			Notes: &notes, Tags: tags, ParentID: &orig.ID, PatchInfo: json.RawMessage(`{"tool":"revanced"}`),
		})
		if err != nil || got == nil {
			t.Fatalf("UpdateAndroidVersionMeta = %v, %v", got, err)
		}

		if got.ParentID != orig.ID || got.Notes != notes || !slices.Equal(got.Tags, tags) || string(got.PatchInfo) != `{"tool": "revanced"}` {
			t.Fatalf("meta = parent %q notes %q tags %v patch %s", got.ParentID, got.Notes, got.Tags, got.PatchInfo)
		}

		foreign := androidTestVersion(t, pg, otherRepo, mustApp(t, pg, otherRepo, "org.other"), 1, strings.Repeat("c", 64), "built", "4")
		if _, err := pg.UpdateAndroidVersionMeta(ctx, repoID, patched.ID, models.AndroidVersionMeta{ParentID: &foreign.ID}); !errors.Is(err, ErrAndroidBadParent) {
			t.Fatalf("foreign parent accepted: %v", err)
		}

		if _, err := pg.UpdateAndroidVersionMeta(ctx, repoID, patched.ID, models.AndroidVersionMeta{ParentID: &patched.ID}); !errors.Is(err, ErrAndroidBadParent) {
			t.Fatalf("self parent accepted: %v", err)
		}

		if got, _ := pg.UpdateAndroidVersionMeta(ctx, otherRepo, patched.ID, models.AndroidVersionMeta{Notes: &notes}); got != nil {
			t.Fatal("version of another repository was updated")
		}

		found, err := pg.FindAndroidOriginal(ctx, repoID, "org.example", 10)
		if err != nil || found == nil || found.ID != orig.ID {
			t.Fatalf("FindAndroidOriginal = %v, %v", found, err)
		}
	})

	t.Run("app meta, tags and signer use", func(t *testing.T) {
		policy, appNotes := models.AndroidPolicyReject, "my notes"

		got, err := pg.UpdateAndroidAppMeta(ctx, repoID, app.ID, models.AndroidAppMeta{
			SignerPolicy: &policy, Notes: &appNotes, Tags: []string{"patched"},
		})
		if err != nil || got == nil || got.SignerPolicy != policy || got.Notes != appNotes {
			t.Fatalf("UpdateAndroidAppMeta = %+v, %v", got, err)
		}

		if got, _ := pg.UpdateAndroidAppMeta(ctx, otherRepo, app.ID, models.AndroidAppMeta{Notes: &appNotes}); got != nil {
			t.Fatal("app of another repository was updated")
		}

		tags, err := pg.ListAndroidTags(ctx, repoID)
		if err != nil || len(tags) != 3 {
			t.Fatalf("tags = %v, %v", tags, err)
		}

		use, err := pg.ListAndroidAppSignerUse(ctx, repoID, app.ID)
		if err != nil || len(use) != 2 {
			t.Fatalf("signer use = %v, %v", use, err)
		}

		if _, n, _ := pg.ListAndroidApps(ctx, repoID, models.AndroidAppFilter{Tags: []string{"patched"}}); n != 1 {
			t.Fatalf("tag filter matched %d", n)
		}
	})

	t.Run("signers: scoped trust wins and upsert keeps label", func(t *testing.T) {
		key := strings.Repeat("d", 64)

		if err := pg.UpsertAndroidSigner(ctx, &models.AndroidSigner{RepositoryID: repoID, SHA256: key, Label: "mine", Trust: "mine"}); err != nil {
			t.Fatal(err)
		}

		if err := pg.UpsertAndroidSigner(ctx, &models.AndroidSigner{RepositoryID: repoID, SHA256: key, Trust: "third_party"}); err != nil {
			t.Fatal(err)
		}

		list, _ := pg.ListAndroidSigners(ctx, repoID, "")
		if len(list) != 1 || list[0].Label != "mine" || list[0].Trust != "third_party" {
			t.Fatalf("signers = %+v", list)
		}

		if err := pg.UpsertAndroidSigner(ctx, &models.AndroidSigner{RepositoryID: repoID, SHA256: key, Trust: "untrusted", AppID: app.ID}); err != nil {
			t.Fatal(err)
		}

		trust, found, err := pg.AndroidSignerTrust(ctx, repoID, app.ID, key)
		if err != nil || !found || trust != "untrusted" {
			t.Fatalf("scoped trust = %q %v %v", trust, found, err)
		}

		if _, found, _ := pg.AndroidSignerTrust(ctx, repoID, app.ID, strings.Repeat("e", 64)); found {
			t.Fatal("unknown key reported as known")
		}

		if ok, _ := pg.DeleteAndroidSigner(ctx, otherRepo, list[0].ID); ok {
			t.Fatal("signer deleted through another repository")
		}
	})

	t.Run("files and icons follow the version", func(t *testing.T) {
		blobID, sha := androidTestBlob(t, pg, "7", 50)
		f := &models.AndroidFile{VersionID: patched.ID, Kind: "mapping", Name: "mapping.txt", BlobID: blobID, SHA256: sha, SizeBytes: 50}

		if err := pg.CreateAndroidFile(ctx, otherRepo, f); err == nil {
			t.Fatal("file attached through another repository")
		}

		if err := pg.CreateAndroidFile(ctx, repoID, f); err != nil {
			t.Fatal(err)
		}

		if files, _ := pg.ListAndroidFiles(ctx, repoID, patched.ID); len(files) != 1 {
			t.Fatalf("files = %v", files)
		}

		iconBlob, iconSHA := androidTestBlob(t, pg, "8", 10)
		withIcon := &models.AndroidVersion{
			RepositoryID: repoID, AppID: app.ID, PackageName: app.PackageName, VersionCode: 12, FileKind: "apk",
			BlobID: blobID, SHA256: sha, SizeBytes: 50, Filename: "b.apk", VariantKey: "icons", Origin: "built",
		}
		icons := []models.AndroidIcon{{Role: "app", Density: "xxhdpi", Width: 144, Height: 144, Mime: "image/png", BlobID: iconBlob, SHA256: iconSHA, SizeBytes: 10}}

		if err := pg.CreateAndroidVersion(ctx, withIcon, icons); err != nil {
			t.Fatal(err)
		}

		got, err := pg.ListAndroidIcons(ctx, repoID, withIcon.ID)
		if err != nil || len(got) != 1 || got[0].BlobID != iconBlob {
			t.Fatalf("icons = %+v, %v", got, err)
		}

		if ic, _ := pg.GetAndroidIcon(ctx, otherRepo, withIcon.ID, got[0].ID); ic != nil {
			t.Fatal("icon leaked across repositories")
		}

		del, err := pg.DeleteAndroidVersion(ctx, repoID, withIcon.ID)
		if err != nil || del == nil || !slices.Contains(del.BlobSHA256, iconSHA) {
			t.Fatalf("DeleteAndroidVersion = %+v, %v", del, err)
		}

		if got, _ := pg.ListAndroidIcons(ctx, repoID, withIcon.ID); len(got) != 0 {
			t.Fatal("icons survived their version")
		}

		del, err = pg.DeleteAndroidVersion(ctx, repoID, patched.ID)
		if err != nil || del == nil || del.QuotaBytes != 150 || len(del.BlobSHA256) != 2 {
			t.Fatalf("delete patched = %+v, %v", del, err)
		}

		if again, _ := pg.DeleteAndroidVersion(ctx, repoID, patched.ID); again != nil {
			t.Fatal("deleted twice")
		}

		if parent, _ := pg.GetAndroidVersion(ctx, repoID, orig.ID, false); parent == nil {
			t.Fatal("deleting a child removed its parent")
		}
	})

	t.Run("deleting an original clears the parent link of its patched copies", func(t *testing.T) {
		child := androidTestVersion(t, pg, repoID, app, 20, strings.Repeat("f", 64), models.AndroidOriginPatched, "9")

		if _, err := pg.UpdateAndroidVersionMeta(ctx, repoID, child.ID, models.AndroidVersionMeta{ParentID: &orig.ID}); err != nil {
			t.Fatal(err)
		}

		if _, err := pg.DeleteAndroidVersion(ctx, repoID, orig.ID); err != nil {
			t.Fatal(err)
		}

		got, _ := pg.GetAndroidVersion(ctx, repoID, child.ID, false)
		if got == nil || got.ParentID != "" {
			t.Fatalf("child = %+v", got)
		}
	})
}

func mustApp(t *testing.T, pg *PostgresDB, repoID, pkg string) *models.AndroidApp {
	t.Helper()

	a, err := pg.UpsertAndroidApp(context.Background(), repoID, pkg)
	if err != nil {
		t.Fatal(err)
	}

	return a
}
