// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()

	log, err := logger.New("info", "json", "stderr")
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	return log
}

func setupPermissionDB(t *testing.T) (*MockDatabase, string, string) {
	t.Helper()

	ctx := context.Background()
	db := newMockDatabase()

	now := time.Now()

	user := &models.User{
		ID:        "perm-user-1",
		Username:  "permuser",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := db.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	group := &models.Group{
		ID:        "perm-group-1",
		Name:      "permgroup",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := db.CreateGroup(ctx, group); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	if err := db.AddUserToGroup(ctx, user.ID, group.ID); err != nil {
		t.Fatalf("AddUserToGroup: %v", err)
	}

	return db, user.ID, group.ID
}

func makeTarget(id, name, repoFilter, pathPattern string, actions []string) *models.PermissionTarget {
	now := time.Now()

	return &models.PermissionTarget{
		ID:               id,
		Name:             name,
		RepositoryFilter: repoFilter,
		PathPattern:      pathPattern,
		Actions:          actions,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestPermission_MatchingTargetAllows(t *testing.T) {
	ctx := context.Background()
	db, userID, _ := setupPermissionDB(t)
	log := newTestLogger(t)

	target := makeTarget("t1", "allow-read", ".*", ".*", []string{"read"})
	if _, err := db.CreatePermissionTarget(ctx, target); err != nil {
		t.Fatalf("CreatePermissionTarget: %v", err)
	}

	if err := db.CreatePermissionAssignment(ctx, &models.PermissionAssignment{
		PermissionTargetID: target.ID,
		PrincipalType:      "user",
		PrincipalID:        userID,
	}); err != nil {
		t.Fatalf("CreatePermissionAssignment: %v", err)
	}

	eval := auth.NewEvaluator(db, log, false)
	uid := userID
	principal := auth.Principal{UserID: &uid, Username: "permuser"}

	ok, err := eval.Evaluate(ctx, principal, "my-repo", "path/to/file", "read")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !ok {
		t.Error("expected allowed")
	}
}

func TestPermission_GroupInheritedAccess(t *testing.T) {
	ctx := context.Background()
	db, userID, groupID := setupPermissionDB(t)
	log := newTestLogger(t)

	target := makeTarget("t2", "group-write", ".*", ".*", []string{"write"})
	if _, err := db.CreatePermissionTarget(ctx, target); err != nil {
		t.Fatalf("CreatePermissionTarget: %v", err)
	}

	if err := db.CreatePermissionAssignment(ctx, &models.PermissionAssignment{
		PermissionTargetID: target.ID,
		PrincipalType:      "group",
		PrincipalID:        groupID,
	}); err != nil {
		t.Fatalf("CreatePermissionAssignment: %v", err)
	}

	eval := auth.NewEvaluator(db, log, false)
	uid := userID
	principal := auth.Principal{
		UserID:   &uid,
		Username: "permuser",
		GroupIDs: []string{groupID},
	}

	ok, err := eval.Evaluate(ctx, principal, "any-repo", "any/path", "write")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !ok {
		t.Error("expected group-inherited access to be allowed")
	}
}

func TestPermission_RepoFilterMismatch(t *testing.T) {
	ctx := context.Background()
	db, userID, _ := setupPermissionDB(t)
	log := newTestLogger(t)

	target := makeTarget("t3", "only-prod", "^prod-.*", ".*", []string{"read"})
	if _, err := db.CreatePermissionTarget(ctx, target); err != nil {
		t.Fatalf("CreatePermissionTarget: %v", err)
	}

	if err := db.CreatePermissionAssignment(ctx, &models.PermissionAssignment{
		PermissionTargetID: target.ID,
		PrincipalType:      "user",
		PrincipalID:        userID,
	}); err != nil {
		t.Fatalf("CreatePermissionAssignment: %v", err)
	}

	eval := auth.NewEvaluator(db, log, false)
	uid := userID
	principal := auth.Principal{UserID: &uid, Username: "permuser"}

	ok, err := eval.Evaluate(ctx, principal, "staging-repo", "any/path", "read")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if ok {
		t.Error("expected denied due to repo filter mismatch")
	}
}

func TestPermission_ActionMismatch(t *testing.T) {
	ctx := context.Background()
	db, userID, _ := setupPermissionDB(t)
	log := newTestLogger(t)

	target := makeTarget("t4", "read-only", ".*", ".*", []string{"read"})
	if _, err := db.CreatePermissionTarget(ctx, target); err != nil {
		t.Fatalf("CreatePermissionTarget: %v", err)
	}

	if err := db.CreatePermissionAssignment(ctx, &models.PermissionAssignment{
		PermissionTargetID: target.ID,
		PrincipalType:      "user",
		PrincipalID:        userID,
	}); err != nil {
		t.Fatalf("CreatePermissionAssignment: %v", err)
	}

	eval := auth.NewEvaluator(db, log, false)
	uid := userID
	principal := auth.Principal{UserID: &uid, Username: "permuser"}

	ok, err := eval.Evaluate(ctx, principal, "any-repo", "any/path", "write")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if ok {
		t.Error("expected denied due to action mismatch (only read allowed, requested write)")
	}
}

func TestPermission_AnonymousReadEnabled(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()
	log := newTestLogger(t)

	eval := auth.NewEvaluator(db, log, true)
	principal := auth.Principal{IsAnonymous: true}

	ok, err := eval.Evaluate(ctx, principal, "any-repo", "any/path", "read")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !ok {
		t.Error("expected anonymous read allowed when flag=true and anonymous-read target seeded")
	}
}

func TestPermission_AnonymousReadDisabled(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()
	log := newTestLogger(t)

	eval := auth.NewEvaluator(db, log, false)
	principal := auth.Principal{IsAnonymous: true}

	ok, err := eval.Evaluate(ctx, principal, "any-repo", "any/path", "read")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if ok {
		t.Error("expected anonymous read denied when flag=false")
	}
}

func TestPermission_AnonymousWriteAlwaysDenied(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()
	log := newTestLogger(t)

	eval := auth.NewEvaluator(db, log, true)
	principal := auth.Principal{IsAnonymous: true}

	ok, err := eval.Evaluate(ctx, principal, "any-repo", "any/path", "write")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if ok {
		t.Error("expected anonymous write denied even when allowAnonymousRead=true")
	}
}

func TestPermission_LegacyGrantsUnconditionally(t *testing.T) {
	ctx := context.Background()
	db := newMockDatabase()
	log := newTestLogger(t)

	eval := auth.NewEvaluator(db, log, false)
	principal := auth.Principal{
		Username: "system/legacy-api-key",
		IsLegacy: true,
	}

	ok, err := eval.Evaluate(ctx, principal, "any-repo", "any/path", "delete")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if !ok {
		t.Error("expected legacy principal to be unconditionally allowed")
	}
}
