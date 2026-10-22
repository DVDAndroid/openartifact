// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package database

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/openartifact/openartifact/internal/database/models"
)

// storeScenario is an app with an original, a patched copy under another key
// and a newer patched version, in one of two repositories.
type storeScenario struct {
	t                     *testing.T // the running step's
	pg                    *PostgresDB
	repoID, otherRepo     string
	app                   *models.AndroidApp
	orig, patched, newest *models.AndroidVersion
}

func key(c string) string { return strings.Repeat(c, 64) }

func TestAndroidStore(t *testing.T) {
	pg := newQuotaTestDB(t, 4)
	s := &storeScenario{pg: pg, repoID: androidTestRepo(t, pg, "apps"), otherRepo: androidTestRepo(t, pg, "other")}

	var err error

	if s.app, err = pg.UpsertAndroidApp(t.Context(), s.repoID, "org.example"); err != nil {
		t.Fatal(err)
	}

	again, err := pg.UpsertAndroidApp(t.Context(), s.repoID, "org.example")
	if err != nil || again.ID != s.app.ID {
		t.Fatalf("upsert is not idempotent: %v %v", again, err)
	}

	s.orig = androidTestVersion(t, pg, s.repoID, s.app, 10, key("a"), models.AndroidOriginOriginal, "1")
	s.patched = androidTestVersion(t, pg, s.repoID, s.app, 10, key("b"), models.AndroidOriginPatched, "2")
	s.newest = androidTestVersion(t, pg, s.repoID, s.app, 11, key("b"), models.AndroidOriginPatched, "3")

	for _, step := range []struct {
		name string
		run  func()
	}{
		{"same variant is a duplicate, another signer is not", s.duplicate},
		{"versions are scoped to their repository", s.scoping},
		{"list apps aggregates versions", s.listApps},
		{"latest honours sdk, abi and signer", s.latest},
		{"download needs the exact file name of the variant", s.download},
		{"meta updates tags, parent and notes", s.versionMeta},
		{"parent must be a version of the same repository", s.parents},
		{"app meta, tags and signer use", s.appMeta},
		{"signers: scoped trust wins and upsert keeps label", s.signers},
		{"files and icons follow the version", s.filesAndIcons},
		{"deleting a version keeps its parent", s.deleteChild},
		{"deleting an original clears the parent link of its patched copies", s.deleteParent},
	} {
		t.Run(step.name, func(t *testing.T) {
			s.t = t
			step.run()
		})
	}
}

func (s *storeScenario) duplicate() {
	dup := *s.orig
	dup.ID = ""

	if err := s.pg.CreateAndroidVersion(s.t.Context(), &dup, nil); !errors.Is(err, ErrDuplicate) {
		s.t.Fatalf("err = %v, want ErrDuplicate", err)
	}
}

func (s *storeScenario) scoping() {
	got, err := s.pg.GetAndroidVersion(s.t.Context(), s.otherRepo, s.orig.ID, true)
	if err != nil || got != nil {
		s.t.Fatalf("version leaked across repositories: %v %v", got, err)
	}

	got, err = s.pg.GetAndroidVersion(s.t.Context(), s.repoID, s.orig.ID, true)
	if err != nil || got == nil || got.ID != s.orig.ID {
		s.t.Fatalf("GetAndroidVersion = %v, %v", got, err)
	}

	if string(got.Entries) != "[]" || string(got.Manifest) != "{}" {
		s.t.Fatalf("detail columns = %s / %s", got.Entries, got.Manifest)
	}
}

func (s *storeScenario) listApps() {
	apps, total, err := s.pg.ListAndroidApps(s.t.Context(), s.repoID, models.AndroidAppFilter{})
	if err != nil || total != 1 || len(apps) != 1 {
		s.t.Fatalf("ListAndroidApps = %d apps, total %d, %v", len(apps), total, err)
	}

	a := apps[0]
	if a.VersionCount != 3 || a.LatestID != s.newest.ID || a.LatestCode != 11 || a.TotalSize != 300 {
		s.t.Fatalf("summary = %+v", a)
	}

	if !slices.Equal(a.Origins, []string{"original", "patched"}) || len(a.Signers) != 2 {
		s.t.Fatalf("origins %v signers %v", a.Origins, a.Signers)
	}

	for _, f := range []models.AndroidAppFilter{{Origin: "original"}, {Signer: key("a")}, {Query: "org.exa"}, {Query: "demo"}, {Query: "dimost"}} {
		if _, n, err := s.pg.ListAndroidApps(s.t.Context(), s.repoID, f); err != nil || n != 1 {
			s.t.Fatalf("filter %+v: total %d, %v", f, n, err)
		}
	}

	for _, f := range []models.AndroidAppFilter{{Origin: "built"}, {Query: "100%"}, {Tags: []string{"nope"}}} {
		if _, n, err := s.pg.ListAndroidApps(s.t.Context(), s.repoID, f); err != nil || n != 0 {
			s.t.Fatalf("filter %+v matched: total %d, %v", f, n, err)
		}
	}

	if _, n, err := s.pg.ListAndroidApps(s.t.Context(), s.otherRepo, models.AndroidAppFilter{}); err != nil || n != 0 {
		s.t.Fatalf("other repository sees %d apps (%v)", n, err)
	}
}

func (s *storeScenario) latest() {
	for _, c := range []struct {
		name string
		f    AndroidLatestFilter
		want string
	}{
		{"any", AndroidLatestFilter{}, s.newest.ID},
		{"signer", AndroidLatestFilter{Signer: key("a")}, s.orig.ID},
		{"abi match", AndroidLatestFilter{ABI: "arm64-v8a"}, s.newest.ID},
		{"abi miss", AndroidLatestFilter{ABI: "x86"}, ""},
		{"sdk too old", AndroidLatestFilter{SDK: 19}, ""},
		{"origin", AndroidLatestFilter{Origin: "original"}, s.orig.ID},
	} {
		got, err := s.pg.LatestAndroidVersion(s.t.Context(), s.repoID, "org.example", c.f)
		if err != nil {
			s.t.Fatal(err)
		}

		id := ""
		if got != nil {
			id = got.ID
		}

		if id != c.want {
			s.t.Errorf("%s: got %q, want %q", c.name, id, c.want)
		}
	}
}

func (s *storeScenario) download() {
	got, err := s.pg.GetAndroidDownload(s.t.Context(), s.repoID, "org.example", 10, s.patched.Filename)
	if err != nil || got == nil || got.ID != s.patched.ID {
		s.t.Fatalf("GetAndroidDownload = %v, %v", got, err)
	}

	for name, c := range map[string]struct {
		code int64
		file string
	}{"unknown versionCode": {99, s.patched.Filename}, "another signer's name": {10, "a-nope.apk"}} {
		if got, _ := s.pg.GetAndroidDownload(s.t.Context(), s.repoID, "org.example", c.code, c.file); got != nil {
			s.t.Fatalf("%s resolved to %s", name, got.ID)
		}
	}
}

func (s *storeScenario) versionMeta() {
	notes, tags := "patched with x", []string{"keep", "device:pixel"}

	got, err := s.pg.UpdateAndroidVersionMeta(s.t.Context(), s.repoID, s.patched.ID, models.AndroidVersionMeta{
		Notes: &notes, Tags: tags, ParentID: &s.orig.ID, PatchInfo: json.RawMessage(`{"tool":"revanced"}`),
	})
	if err != nil || got == nil {
		s.t.Fatalf("UpdateAndroidVersionMeta = %v, %v", got, err)
	}

	if got.ParentID != s.orig.ID || got.Notes != notes || !slices.Equal(got.Tags, tags) || string(got.PatchInfo) != `{"tool": "revanced"}` {
		s.t.Fatalf("meta = parent %q notes %q tags %v patch %s", got.ParentID, got.Notes, got.Tags, got.PatchInfo)
	}

	if got, _ := s.pg.UpdateAndroidVersionMeta(s.t.Context(), s.otherRepo, s.patched.ID, models.AndroidVersionMeta{Notes: &notes}); got != nil {
		s.t.Fatal("version of another repository was updated")
	}

	found, err := s.pg.FindAndroidOriginal(s.t.Context(), s.repoID, "org.example", 10)
	if err != nil || found == nil || found.ID != s.orig.ID {
		s.t.Fatalf("FindAndroidOriginal = %v, %v", found, err)
	}
}

func (s *storeScenario) parents() {
	foreign := androidTestVersion(s.t, s.pg, s.otherRepo, mustApp(s.t, s.pg, s.otherRepo, "org.other"), 1, key("c"), "built", "4")

	for name, parent := range map[string]string{"foreign": foreign.ID, "self": s.patched.ID} {
		_, err := s.pg.UpdateAndroidVersionMeta(s.t.Context(), s.repoID, s.patched.ID, models.AndroidVersionMeta{ParentID: &parent})
		if !errors.Is(err, ErrAndroidBadParent) {
			s.t.Errorf("%s parent accepted: %v", name, err)
		}
	}
}

func (s *storeScenario) appMeta() {
	policy, appNotes := models.AndroidPolicyReject, "my notes"

	got, err := s.pg.UpdateAndroidAppMeta(s.t.Context(), s.repoID, s.app.ID, models.AndroidAppMeta{
		SignerPolicy: &policy, Notes: &appNotes, Tags: []string{"patched"},
	})
	if err != nil || got == nil || got.SignerPolicy != policy || got.Notes != appNotes {
		s.t.Fatalf("UpdateAndroidAppMeta = %+v, %v", got, err)
	}

	if got, _ := s.pg.UpdateAndroidAppMeta(s.t.Context(), s.otherRepo, s.app.ID, models.AndroidAppMeta{Notes: &appNotes}); got != nil {
		s.t.Fatal("app of another repository was updated")
	}

	tags, err := s.pg.ListAndroidTags(s.t.Context(), s.repoID)
	if err != nil || len(tags) != 3 {
		s.t.Fatalf("tags = %v, %v", tags, err)
	}

	use, err := s.pg.ListAndroidAppSignerUse(s.t.Context(), s.repoID, s.app.ID)
	if err != nil || len(use) != 2 {
		s.t.Fatalf("signer use = %v, %v", use, err)
	}

	if _, n, _ := s.pg.ListAndroidApps(s.t.Context(), s.repoID, models.AndroidAppFilter{Tags: []string{"patched"}}); n != 1 {
		s.t.Fatalf("tag filter matched %d", n)
	}
}

func (s *storeScenario) signers() {
	k := key("d")

	if err := s.pg.UpsertAndroidSigner(s.t.Context(), &models.AndroidSigner{RepositoryID: s.repoID, SHA256: k, Label: "mine", Trust: "mine"}); err != nil {
		s.t.Fatal(err)
	}

	if err := s.pg.UpsertAndroidSigner(s.t.Context(), &models.AndroidSigner{RepositoryID: s.repoID, SHA256: k, Trust: "third_party"}); err != nil {
		s.t.Fatal(err)
	}

	list, _ := s.pg.ListAndroidSigners(s.t.Context(), s.repoID, "")
	if len(list) != 1 || list[0].Label != "mine" || list[0].Trust != "third_party" {
		s.t.Fatalf("signers = %+v", list)
	}

	if err := s.pg.UpsertAndroidSigner(s.t.Context(), &models.AndroidSigner{RepositoryID: s.repoID, SHA256: k, Trust: "untrusted", AppID: s.app.ID}); err != nil {
		s.t.Fatal(err)
	}

	trust, found, err := s.pg.AndroidSignerTrust(s.t.Context(), s.repoID, s.app.ID, k)
	if err != nil || !found || trust != "untrusted" {
		s.t.Fatalf("scoped trust = %q %v %v", trust, found, err)
	}

	// Before the app exists only repository-wide registrations apply.
	if trust, found, _ := s.pg.AndroidSignerTrust(s.t.Context(), s.repoID, "", k); !found || trust != "third_party" {
		s.t.Fatalf("repository-wide trust = %q %v", trust, found)
	}

	if _, found, _ := s.pg.AndroidSignerTrust(s.t.Context(), s.repoID, s.app.ID, key("e")); found {
		s.t.Fatal("unknown key reported as known")
	}

	if ok, _ := s.pg.DeleteAndroidSigner(s.t.Context(), s.otherRepo, list[0].ID); ok {
		s.t.Fatal("signer deleted through another repository")
	}
}

func (s *storeScenario) filesAndIcons() {
	blobID, sha := androidTestBlob(s.t, s.pg, "7", 50)
	f := &models.AndroidFile{VersionID: s.patched.ID, Kind: "mapping", Name: "mapping.txt", BlobID: blobID, SHA256: sha, SizeBytes: 50}

	if err := s.pg.CreateAndroidFile(s.t.Context(), s.otherRepo, f); err == nil {
		s.t.Fatal("file attached through another repository")
	}

	if err := s.pg.CreateAndroidFile(s.t.Context(), s.repoID, f); err != nil {
		s.t.Fatal(err)
	}

	if files, _ := s.pg.ListAndroidFiles(s.t.Context(), s.repoID, s.patched.ID); len(files) != 1 {
		s.t.Fatalf("files = %v", files)
	}

	iconBlob, iconSHA := androidTestBlob(s.t, s.pg, "8", 10)
	withIcon := &models.AndroidVersion{
		RepositoryID: s.repoID, AppID: s.app.ID, PackageName: s.app.PackageName, VersionCode: 12, FileKind: "apk",
		BlobID: blobID, SHA256: sha, SizeBytes: 50, Filename: "b.apk", VariantKey: "icons", Origin: "built",
	}
	icons := []models.AndroidIcon{{Role: "app", Density: "xxhdpi", Width: 144, Height: 144, Mime: "image/png", BlobID: iconBlob, SHA256: iconSHA, SizeBytes: 10}}

	if err := s.pg.CreateAndroidVersion(s.t.Context(), withIcon, icons); err != nil {
		s.t.Fatal(err)
	}

	got, err := s.pg.ListAndroidIcons(s.t.Context(), s.repoID, withIcon.ID)
	if err != nil || len(got) != 1 || got[0].BlobID != iconBlob {
		s.t.Fatalf("icons = %+v, %v", got, err)
	}

	if ic, _ := s.pg.GetAndroidIcon(s.t.Context(), s.otherRepo, withIcon.ID, got[0].ID); ic != nil {
		s.t.Fatal("icon leaked across repositories")
	}

	del, err := s.pg.DeleteAndroidVersion(s.t.Context(), s.repoID, withIcon.ID)
	if err != nil || del == nil || !slices.Contains(del.BlobSHA256, iconSHA) {
		s.t.Fatalf("DeleteAndroidVersion = %+v, %v", del, err)
	}

	if got, _ := s.pg.ListAndroidIcons(s.t.Context(), s.repoID, withIcon.ID); len(got) != 0 {
		s.t.Fatal("icons survived their version")
	}
}

func (s *storeScenario) deleteChild() {
	del, err := s.pg.DeleteAndroidVersion(s.t.Context(), s.repoID, s.patched.ID)
	if err != nil || del == nil || del.QuotaBytes != 150 || del.QuotaCount != 2 || len(del.BlobSHA256) != 2 {
		s.t.Fatalf("delete patched = %+v, %v", del, err)
	}

	if again, _ := s.pg.DeleteAndroidVersion(s.t.Context(), s.repoID, s.patched.ID); again != nil {
		s.t.Fatal("deleted twice")
	}

	if parent, _ := s.pg.GetAndroidVersion(s.t.Context(), s.repoID, s.orig.ID, false); parent == nil {
		s.t.Fatal("deleting a child removed its parent")
	}
}

func (s *storeScenario) deleteParent() {
	child := androidTestVersion(s.t, s.pg, s.repoID, s.app, 20, key("f"), models.AndroidOriginPatched, "9")

	if _, err := s.pg.UpdateAndroidVersionMeta(s.t.Context(), s.repoID, child.ID, models.AndroidVersionMeta{ParentID: &s.orig.ID}); err != nil {
		s.t.Fatal(err)
	}

	if _, err := s.pg.DeleteAndroidVersion(s.t.Context(), s.repoID, s.orig.ID); err != nil {
		s.t.Fatal(err)
	}

	got, _ := s.pg.GetAndroidVersion(s.t.Context(), s.repoID, child.ID, false)
	if got == nil || got.ParentID != "" {
		s.t.Fatalf("child = %+v", got)
	}
}
