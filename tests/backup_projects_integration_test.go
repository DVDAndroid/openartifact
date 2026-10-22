// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package integration_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/openartifact/openartifact/internal/backup"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/logger"
)

func newBackupEngine(t *testing.T, db database.Database) *backup.Engine {
	t.Helper()

	log, err := logger.New("error", "json", "stderr")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	blobs, err := storage.NewFilesystemStore(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}

	return backup.NewEngine(db, blobs, log, filepath.Join(dir, "backups"))
}

// TestBackupRestore_ProjectsRealDB backs up projects with policy fields,
// members, group attachments and repositories on one real database and
// restores them into another one: ids are remapped, nothing is lost.
func TestBackupRestore_ProjectsRealDB(t *testing.T) {
	ctx := context.Background()
	src := freshPostgres(t, 5)
	dst := freshPostgres(t, 5)

	owner, err := src.CreateUser(ctx, &models.User{Username: "bk-owner"})
	if err != nil {
		t.Fatal(err)
	}

	dev, err := src.CreateUser(ctx, &models.User{Username: "bk-dev"})
	if err != nil {
		t.Fatal(err)
	}

	team, err := src.CreateGroup(ctx, &models.Group{Name: "bk-team"})
	if err != nil {
		t.Fatal(err)
	}

	formats := []string{"raw", "npm"}
	acme := &models.Project{
		Name: "bk-acme", OwnerID: &owner.ID, AllowedFormats: formats,
		AllowedRepoTypes: []string{"hosted", "proxy"}, RepoNamePrefix: "bk-acme-",
		AllowPublicRepos: true, MaxRepositories: 7, TokenMaxTTLDays: 30,
	}
	if err := src.CreateProject(ctx, acme); err != nil {
		t.Fatal(err)
	}

	if err := src.AddProjectMember(ctx, acme.ID, dev.ID, models.ProjectRoleDeveloper); err != nil {
		t.Fatal(err)
	}

	if err := src.SetProjectGroup(ctx, acme.ID, team.ID, models.ProjectRoleMaintainer, &owner.ID); err != nil {
		t.Fatal(err)
	}

	repo := &models.Repository{Name: "bk-acme-raw", Format: "raw", RepoType: "hosted", ProjectID: &acme.ID}
	if err := src.CreateRepository(ctx, repo); err != nil {
		t.Fatal(err)
	}

	job := &models.BackupJob{Status: "pending", Type: "full"}
	if err := src.CreateBackupJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if err := newBackupEngine(t, src).RunBackup(ctx, job.ID, "full"); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	done, err := src.GetBackupJob(ctx, job.ID)
	if err != nil || done.Status != "completed" {
		t.Fatalf("backup job = %+v, %v", done, err)
	}

	if err := newBackupEngine(t, dst).RunRestore(ctx, done.StoragePath); err != nil {
		t.Fatalf("RunRestore: %v", err)
	}

	got, err := dst.GetProjectByName(ctx, "bk-acme")
	if err != nil || got == nil {
		t.Fatalf("project not restored: %v", err)
	}

	if got.ID == acme.ID || got.Slug != "bk-acme" || !slices.Equal(got.AllowedFormats, formats) ||
		!slices.Equal(got.AllowedRepoTypes, []string{"hosted", "proxy"}) || got.RepoNamePrefix != "bk-acme-" ||
		!got.AllowPublicRepos || got.MaxRepositories != 7 || got.TokenMaxTTLDays != 30 {
		t.Fatalf("restored project = %+v", got)
	}

	newOwner, err := dst.GetUserByUsername(ctx, "bk-owner")
	if err != nil || newOwner == nil || got.OwnerID == nil || *got.OwnerID != newOwner.ID {
		t.Fatalf("owner not remapped: %v (project owner %v)", err, got.OwnerID)
	}

	newDev, err := dst.GetUserByUsername(ctx, "bk-dev")
	if err != nil || newDev == nil {
		t.Fatalf("dev user not restored: %v", err)
	}

	member, err := dst.GetProjectMember(ctx, got.ID, newDev.ID)
	if err != nil || member == nil || member.Role != models.ProjectRoleDeveloper {
		t.Fatalf("member = %+v, %v; want developer", member, err)
	}

	groups, err := dst.ListProjectGroups(ctx, got.ID)
	if err != nil || len(groups) != 1 || groups[0].Role != models.ProjectRoleMaintainer || groups[0].GroupName != "bk-team" || groups[0].GroupID == team.ID {
		t.Fatalf("project groups = %+v, %v; want bk-team maintainer with a new group id", groups, err)
	}

	if groups[0].CreatedBy == nil || *groups[0].CreatedBy != newOwner.ID {
		t.Fatalf("project group created_by = %v, want remapped owner %s", groups[0].CreatedBy, newOwner.ID)
	}

	restored, err := dst.GetRepositoryByName(ctx, "bk-acme-raw")
	if err != nil || restored == nil || restored.ProjectID == nil || *restored.ProjectID != got.ID {
		t.Fatalf("repository = %+v, %v; want moved into the restored project", restored, err)
	}
}
