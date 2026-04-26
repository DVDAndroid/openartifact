// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SearchResult struct {
	Repository string            `json:"repository"`
	Format     string            `json:"format"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Path       string            `json:"path"`
	SHA256     string            `json:"sha256"`
	Size       int64             `json:"size"`
	CreatedAt  time.Time         `json:"created_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Executor struct {
	db *sql.DB
}

func NewExecutor(db *sql.DB) *Executor {
	return &Executor{db: db}
}

// unifiedSearchView returns a subquery joining every format table (through the
// global content-addressed blobs table for sha256/size) into a single shape:
// repository_id, format, name, version, created_at, path, sha256, size,
// group_id, artifact_id_coord, scope. group_id/artifact_id_coord are populated
// only for maven (used to enrich SearchResult.Metadata); scope only for npm.
func unifiedSearchView() string {
	return `(` + strings.Join([]string{
		`SELECT cc.repository_id, 'cargo'::text AS format, cc.name, cc.version, cc.created_at,
			cc.name || '-' || cc.version || '.crate' AS path,
			COALESCE(b.sha256, '') AS sha256, COALESCE(b.size_bytes, 0) AS size,
			NULL::text AS group_id, NULL::text AS artifact_id_coord, NULL::text AS scope
		FROM cargo_crates cc LEFT JOIN blobs b ON b.id = cc.blob_id`,
		`SELECT cp.repository_id, 'conda'::text, cp.name, cp.version, cp.created_at,
			''::text,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM conda_packages cp LEFT JOIN blobs b ON b.id = cp.blob_id`,
		`SELECT co.repository_id, 'conan'::text, co.name, co.version, co.created_at,
			''::text,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM conan_packages co LEFT JOIN blobs b ON b.id = co.blob_id`,
		`SELECT dt.repository_id, 'docker'::text, dt.name, ''::text, dt.created_at,
			''::text,
			COALESCE(regexp_replace(dm.digest, '^sha256:', ''), ''),
			COALESCE(dm.size, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM docker_tags dt
		LEFT JOIN docker_manifests dm
			ON dm.repository_id = dt.repository_id AND dm.digest = dt.manifest_digest`,
		`SELECT gm.repository_id, 'gomod'::text, gm.module_path, gm.version, gm.created_at,
			''::text,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM gomod_modules gm LEFT JOIN blobs b ON b.id = gm.zip_blob_id`,
		`SELECT hc.repository_id, 'helm'::text, hc.name, hc.version, hc.created_at,
			hc.name || '-' || hc.version || '.tgz',
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM helm_charts hc LEFT JOIN blobs b ON b.id = hc.blob_id`,
		`SELECT iv.repository_id, 'ivy'::text, iv.organisation || '/' || iv.module, iv.revision, iv.created_at,
			iv.artifact_name || '.' || iv.artifact_ext,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM ivy_artifacts iv LEFT JOIN blobs b ON b.id = iv.blob_id`,
		`SELECT ma.repository_id, 'maven'::text, ma.group_id || ':' || ma.artifact_id_coord, ma.version, ma.created_at,
			ma.repo_path,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			ma.group_id, ma.artifact_id_coord, NULL::text
		FROM maven_artifacts ma LEFT JOIN blobs b ON b.id = ma.blob_id`,
		`SELECT np.repository_id, 'npm'::text, np.full_name, nv.version, nv.created_at,
			nv.tarball_filename,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULLIF(np.scope, '')
		FROM npm_packages np
		JOIN npm_versions nv ON nv.package_id = np.id
		LEFT JOIN blobs b ON b.id = nv.blob_id`,
		`SELECT ng.repository_id, 'nuget'::text, ng.package_id, ng.version, ng.created_at,
			ng.package_id || '.' || ng.version || '.nupkg',
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM nuget_packages ng LEFT JOIN blobs b ON b.id = ng.blob_id`,
		`SELECT pp.repository_id, 'pypi'::text, pp.name, pp.version, pp.created_at,
			pp.filename,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM pypi_packages pp LEFT JOIN blobs b ON b.id = pp.blob_id`,
		`SELECT ra.repository_id, 'raw'::text, ra.path, ''::text, ra.created_at,
			ra.path,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM raw_artifacts ra LEFT JOIN blobs b ON b.id = ra.blob_id`,
		`SELECT rg.repository_id, 'rubygems'::text, rg.name, rg.version, rg.created_at,
			rg.name || '-' || rg.version || '.gem',
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM rubygems_gems rg LEFT JOIN blobs b ON b.id = rg.blob_id`,
		`SELECT sa.repository_id, 'sbt'::text, sa.group_id || '/' || sa.artifact_id, sa.version, sa.created_at,
			sa.filename,
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM sbt_artifacts sa LEFT JOIN blobs b ON b.id = sa.blob_id`,
		`SELECT rp.repository_id, 'rpm'::text, rp.name, rp.version, rp.created_at,
			COALESCE(rp.filename, ''),
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM rpm_packages rp LEFT JOIN blobs b ON b.id = rp.blob_id`,
		`SELECT dp.repository_id, 'deb'::text, dp.name, dp.version, dp.created_at,
			COALESCE(dp.filename, ''),
			COALESCE(b.sha256, ''), COALESCE(b.size_bytes, 0),
			NULL::text, CAST(NULL AS text), NULL::text
		FROM deb_packages dp LEFT JOIN blobs b ON b.id = dp.blob_id`,
	}, "\n\tUNION ALL\n\t") + `)`
}

// CountResults returns how many rows the filter matches, scoped to the
// caller's readable repositories. An empty readableRepos set (no permission
// on anything) always yields 0 — it must never fall back to counting all rows.
func (e *Executor) CountResults(ctx context.Context, filter SearchFilter, readableRepos map[uuid.UUID]struct{}) (int64, error) {
	if len(readableRepos) == 0 {
		return 0, nil
	}

	where, args := buildWhereClause(filter, readableRepos)

	q := `SELECT COUNT(*) FROM ` + unifiedSearchView() + ` AS s
		JOIN repositories r ON r.id = s.repository_id`
	if where != "" {
		q += " WHERE " + where
	}

	var count int64
	if err := e.db.QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count query: %w", err)
	}

	return count, nil
}

// Search runs the filter scoped to readableRepos. Repo scoping is enforced
// twice: at the SQL level (repository_id = ANY($n)) and again in Go after
// scanning, so a caller with no readable repositories always gets an empty
// result set even if the SQL-level filter were ever bypassed or misapplied.
func (e *Executor) Search(
	ctx context.Context,
	filter SearchFilter,
	limit, offset int,
	readableRepos map[uuid.UUID]struct{},
) ([]SearchResult, error) {
	if len(readableRepos) == 0 {
		return nil, nil
	}

	where, args := buildWhereClause(filter, readableRepos)

	orderBy := "s.created_at DESC"

	limitN := len(args) + 1
	offsetN := len(args) + 2
	args = append(args, limit, offset)

	q := fmt.Sprintf(`SELECT
		s.repository_id,
		r.name AS repository,
		s.format,
		s.name,
		s.version,
		s.path,
		s.sha256,
		s.size,
		s.created_at,
		s.group_id,
		s.artifact_id_coord,
		s.scope
	FROM `+unifiedSearchView()+` AS s
	JOIN repositories r ON r.id = s.repository_id
	%s
	ORDER BY %s
	LIMIT $%d OFFSET $%d`,
		whereClause(where),
		orderBy,
		limitN,
		offsetN,
	)

	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SearchResult

	for rows.Next() {
		var (
			repoIDRaw  string
			sr         SearchResult
			groupID    sql.NullString
			artifactID sql.NullString
			scope      sql.NullString
		)

		if err := rows.Scan(&repoIDRaw, &sr.Repository, &sr.Format, &sr.Name, &sr.Version,
			&sr.Path, &sr.SHA256, &sr.Size, &sr.CreatedAt, &groupID, &artifactID, &scope); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}

		repoID, err := uuid.Parse(repoIDRaw)
		if err != nil {
			continue
		}

		if _, ok := readableRepos[repoID]; !ok {
			continue
		}

		sr.Metadata = map[string]string{}
		if groupID.Valid {
			sr.Metadata["groupId"] = groupID.String
		}

		if artifactID.Valid {
			sr.Metadata["artifactId"] = artifactID.String
		}

		if scope.Valid {
			sr.Metadata["scope"] = scope.String
		}

		results = append(results, sr)
	}

	return results, rows.Err()
}

func whereClause(where string) string {
	if where == "" {
		return ""
	}

	return "WHERE " + where
}

// buildWhereClause builds the filter predicate for both CountResults and
// Search. readableRepos must be non-empty (callers short-circuit before
// calling this) — its keys are always ANDed in as an explicit repository_id
// scope so a caller can never see rows from a repository they can't read.
func buildWhereClause(filter SearchFilter, readableRepos map[uuid.UUID]struct{}) (string, []any) {
	var (
		clauses []string
		args    []any
	)

	if filter.Name != "" {
		n := len(args) + 1
		clauses = append(clauses, fmt.Sprintf("s.name ILIKE $%d", n))
		args = append(args, "%"+filter.Name+"%")
	}

	if filter.FreeText != "" {
		n := len(args) + 1
		clauses = append(clauses, fmt.Sprintf("s.name ILIKE $%d", n))
		args = append(args, "%"+filter.FreeText+"%")
	}

	if filter.SHA256Prefix != "" {
		n := len(args) + 1
		clauses = append(clauses, fmt.Sprintf("s.sha256 LIKE $%d", n))
		args = append(args, filter.SHA256Prefix+"%")
	}

	if len(filter.Formats) > 0 {
		placeholders := make([]string, len(filter.Formats))
		for i, f := range filter.Formats {
			n := len(args) + 1
			placeholders[i] = fmt.Sprintf("$%d", n)
			args = append(args, f)
		}

		clauses = append(clauses, fmt.Sprintf("s.format IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(filter.Repos) > 0 {
		placeholders := make([]string, len(filter.Repos))
		for i, rn := range filter.Repos {
			n := len(args) + 1
			placeholders[i] = fmt.Sprintf("$%d", n)
			args = append(args, rn)
		}

		clauses = append(clauses, fmt.Sprintf("r.name IN (%s)", strings.Join(placeholders, ",")))
	}

	idStrs := make([]string, 0, len(readableRepos))
	for id := range readableRepos {
		idStrs = append(idStrs, id.String())
	}

	n := len(args) + 1
	clauses = append(clauses, fmt.Sprintf("s.repository_id = ANY($%d::uuid[])", n))
	args = append(args, "{"+strings.Join(idStrs, ",")+"}")

	where := strings.Join(clauses, " AND ")

	return where, args
}
