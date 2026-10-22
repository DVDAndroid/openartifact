// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package database

import (
	"strings"
	"testing"
)

const androidTestSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// The android migration widens the format CHECK and its constraints reject
// malformed rows; blobs an Android row points to cannot be deleted.
func TestAndroidSchema(t *testing.T) {
	db := scratchDB(t)
	migrateAll(t, db)

	org := mustScan[string](t, db, `SELECT id FROM organizations LIMIT 1`)
	repo := mustScan[string](t, db,
		`INSERT INTO repositories (name, format, org_id) VALUES ('apps', 'android', $1) RETURNING id`, org)
	blob := mustScan[string](t, db,
		`INSERT INTO blobs (sha256, size_bytes, blob_key) VALUES ($1, 10, $2) RETURNING id`, androidTestSHA, androidTestSHA)
	app := mustScan[string](t, db,
		`INSERT INTO android_apps (repository_id, package_name) VALUES ($1, 'org.example') RETURNING id`, repo)

	insertVersion := func(code int, variant string) error {
		_, err := db.ExecContext(t.Context(), `INSERT INTO android_versions
			(repository_id, app_id, package_name, version_code, blob_id, sha256, size_bytes, filename, variant_key)
			VALUES ($1, $2, 'org.example', $3, $4, $5, 10, 'a.apk', $6)`,
			repo, app, code, blob, androidTestSHA, variant)

		return err
	}

	if err := insertVersion(1, "k1"); err != nil {
		t.Fatalf("insert version: %v", err)
	}

	t.Run("variant key is unique", func(t *testing.T) {
		wantPgCode(t, insertVersion(1, "k1"), "23505", "duplicate variant")

		if err := insertVersion(1, "k2"); err != nil {
			t.Fatalf("second signer for the same versionCode must be allowed: %v", err)
		}
	})

	t.Run("duplicate package per repository", func(t *testing.T) {
		_, err := db.ExecContext(t.Context(), `INSERT INTO android_apps (repository_id, package_name) VALUES ($1, 'org.example')`, repo)
		wantPgCode(t, err, "23505", "duplicate app")
	})

	t.Run("origin check", func(t *testing.T) {
		_, err := db.ExecContext(t.Context(), `UPDATE android_versions SET origin = 'bogus' WHERE package_name = 'org.example'`)
		wantPgCode(t, err, pgCheckViolation, "origin")
	})

	t.Run("signer fingerprint must be lowercase hex", func(t *testing.T) {
		_, err := db.ExecContext(t.Context(), `INSERT INTO android_signers (repository_id, sha256, trust) VALUES ($1, $2, 'mine')`,
			repo, strings.ToUpper(androidTestSHA))
		wantPgCode(t, err, pgCheckViolation, "signer sha")
	})

	t.Run("known key is unique per repository and app", func(t *testing.T) {
		ins := func(appID any) error {
			_, err := db.ExecContext(t.Context(), `INSERT INTO android_signers (repository_id, sha256, trust, app_id) VALUES ($1, $2, 'mine', $3)`,
				repo, androidTestSHA, appID)

			return err
		}

		if err := ins(nil); err != nil {
			t.Fatal(err)
		}

		wantPgCode(t, ins(nil), "23505", "repository-wide duplicate")

		if err := ins(app); err != nil {
			t.Fatalf("same key scoped to an app must be allowed: %v", err)
		}
	})

	t.Run("referenced blob cannot be deleted", func(t *testing.T) {
		_, err := db.ExecContext(t.Context(), `DELETE FROM blobs WHERE id = $1`, blob)
		wantPgCode(t, err, "23001", "blob restrict")
	})

	t.Run("format check still rejects unknown formats", func(t *testing.T) {
		_, err := db.ExecContext(t.Context(), `INSERT INTO repositories (name, format, org_id) VALUES ('x', 'nope', $1)`, org)
		wantPgCode(t, err, pgCheckViolation, "format")
	})
}
