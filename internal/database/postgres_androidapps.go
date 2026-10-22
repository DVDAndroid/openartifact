// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/openartifact/openartifact/internal/database/models"
)

// The android tables are read and written with hand-written SQL on the native
// pgx pool (text[] and jsonb need no adapters there), see migrations/900_android.sql.

// sqlArgs collects positional arguments while a query is assembled.
type sqlArgs struct{ args []any }

// add appends v and returns its placeholder.
func (a *sqlArgs) add(v any) string {
	a.args = append(a.args, v)

	return "$" + strconv.Itoa(len(a.args))
}

// likePattern returns a case-insensitive substring pattern for ILIKE.
func likePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

	return "%" + r.Replace(s) + "%"
}

// rawOr returns raw, or def when raw is empty: jsonb columns are NOT NULL.
func rawOr(raw json.RawMessage, def string) []byte {
	if len(raw) == 0 {
		return []byte(def)
	}

	return raw
}

const androidAppCols = `a.id::text, a.repository_id::text, a.package_name, COALESCE(a.label_override, ''), a.notes, a.tags, a.signer_policy,
	COALESCE(a.upstream_kind, ''), COALESCE(a.upstream_ref, ''), COALESCE(a.upstream_latest, ''), a.created_at, a.updated_at`

func scanAndroidApp(row rowScanner) (*models.AndroidApp, error) {
	a := &models.AndroidApp{}

	if err := row.Scan(&a.ID, &a.RepositoryID, &a.PackageName, &a.LabelOverride, &a.Notes, &a.Tags, &a.SignerPolicy,
		&a.UpstreamKind, &a.UpstreamRef, &a.UpstreamLatest, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err //nolint:wrapcheck // callers wrap with their own context and test pgx.ErrNoRows
	}

	return a, nil
}

// UpsertAndroidApp returns the live app of the repository, creating it first.
func (p *PostgresDB) UpsertAndroidApp(ctx context.Context, repoID, packageName string) (*models.AndroidApp, error) {
	a, err := scanAndroidApp(p.pool.QueryRow(ctx, `
		INSERT INTO android_apps AS a (repository_id, package_name) VALUES ($1, $2)
		ON CONFLICT (repository_id, package_name) WHERE deleted_at IS NULL DO UPDATE SET updated_at = now()
		RETURNING `+androidAppCols, repoID, packageName))
	if err != nil {
		return nil, fmt.Errorf("failed to upsert android app: %w", err)
	}

	return a, nil
}

// GetAndroidApp returns the app with the package name, or nil, nil.
func (p *PostgresDB) GetAndroidApp(ctx context.Context, repoID, packageName string) (*models.AndroidApp, error) {
	a, err := scanAndroidApp(p.pool.QueryRow(ctx,
		`SELECT `+androidAppCols+` FROM android_apps a WHERE a.repository_id = $1 AND a.package_name = $2 AND a.deleted_at IS NULL`,
		repoID, packageName))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android app: %w", err)
	}

	return a, nil
}

// GetAndroidAppByID returns the app of the repository with the id, or nil, nil.
func (p *PostgresDB) GetAndroidAppByID(ctx context.Context, repoID, id string) (*models.AndroidApp, error) {
	if !isUUID(id) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	a, err := scanAndroidApp(p.pool.QueryRow(ctx,
		`SELECT `+androidAppCols+` FROM android_apps a WHERE a.repository_id = $1 AND a.id = $2 AND a.deleted_at IS NULL`,
		repoID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android app: %w", err)
	}

	return a, nil
}

// androidAppSort maps AndroidAppFilter.Sort to its ORDER BY expression and
// default direction.
var androidAppSort = map[string]struct { //nolint:gochecknoglobals // read-only lookup table
	expr string
	desc bool
}{
	"name":     {`lower(COALESCE(NULLIF(a.label_override, ''), lv.label, a.package_name))`, false},
	"updated":  {`a.updated_at`, true},
	"versions": {`COALESCE(agg.cnt, 0)`, true},
	"size":     {`COALESCE(agg.total, 0)`, true},
}

// ListAndroidApps returns the apps of a repository with their aggregates and
// the total number matching the filter.
func (p *PostgresDB) ListAndroidApps(ctx context.Context, repoID string, f models.AndroidAppFilter) ([]models.AndroidAppSummary, int, error) {
	var a sqlArgs

	where := []string{"a.repository_id = " + a.add(repoID), "a.deleted_at IS NULL"}

	if f.Query != "" {
		q := a.add(likePattern(f.Query))
		where = append(where, `(a.package_name ILIKE `+q+` OR a.label_override ILIKE `+q+` OR a.notes ILIKE `+q+
			` OR lv.label ILIKE `+q+` OR EXISTS (SELECT 1 FROM unnest(a.tags) t WHERE t ILIKE `+q+`))`)
	}

	if len(f.Tags) > 0 {
		where = append(where, "a.tags @> "+a.add(f.Tags))
	}

	if f.Origin != "" {
		where = append(where, a.add(f.Origin)+" = ANY(COALESCE(agg.origins, '{}'::text[]))")
	}

	if f.Signer != "" {
		where = append(where, a.add(f.Signer)+" = ANY(COALESCE(agg.signers, '{}'::text[]))")
	}

	order, ok := androidAppSort[f.Sort]
	if !ok {
		order = androidAppSort["updated"]
	}

	dir := "ASC"
	if order.desc != f.Reverse {
		dir = "DESC"
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `SELECT ` + androidAppCols + `,
			COALESCE(NULLIF(a.label_override, ''), lv.label, ''),
			COALESCE(agg.cnt, 0), COALESCE(agg.origins, '{}'::text[]), COALESCE(agg.signers, '{}'::text[]), COALESCE(agg.total, 0),
			COALESCE(lv.id::text, ''), COALESCE(lv.version_name, ''), COALESCE(lv.version_code, 0),
			COALESCE(lv.signer_sha256, ''), COALESCE(lv.origin, ''), COALESCE(lv.created_at, a.updated_at),
			count(*) OVER()
		FROM android_apps a
		LEFT JOIN LATERAL (
			SELECT v.id, v.label, v.version_name, v.version_code, v.signer_sha256, v.origin, v.created_at
			FROM android_versions v WHERE v.app_id = a.id AND v.deleted_at IS NULL
			ORDER BY v.version_code DESC, v.created_at DESC LIMIT 1) lv ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::int AS cnt, array_agg(DISTINCT v.origin) AS origins,
				array_agg(DISTINCT v.signer_sha256) AS signers, sum(v.size_bytes)::bigint AS total
			FROM android_versions v WHERE v.app_id = a.id AND v.deleted_at IS NULL) agg ON true
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + order.expr + ` ` + dir + `, a.package_name
		LIMIT ` + a.add(limit) + ` OFFSET ` + a.add(max(f.Offset, 0))

	rows, err := p.pool.Query(ctx, query, a.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list android apps: %w", err)
	}
	defer rows.Close()

	var (
		out   []models.AndroidAppSummary
		total int
	)

	for rows.Next() {
		var s models.AndroidAppSummary

		if err := rows.Scan(&s.ID, &s.RepositoryID, &s.PackageName, &s.LabelOverride, &s.Notes, &s.Tags, &s.SignerPolicy,
			&s.UpstreamKind, &s.UpstreamRef, &s.UpstreamLatest, &s.CreatedAt, &s.UpdatedAt,
			&s.Label, &s.VersionCount, &s.Origins, &s.Signers, &s.TotalSize,
			&s.LatestID, &s.LatestName, &s.LatestCode, &s.LatestSigner, &s.LatestOrigin, &s.LatestAt, &total); err != nil {
			return nil, 0, fmt.Errorf("failed to scan android app: %w", err)
		}

		out = append(out, s)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to list android apps: %w", err)
	}

	return out, total, nil
}

// UpdateAndroidAppMeta applies the non-nil fields of m and returns the app, or
// nil, nil when it does not exist in the repository.
func (p *PostgresDB) UpdateAndroidAppMeta(ctx context.Context, repoID, id string, m models.AndroidAppMeta) (*models.AndroidApp, error) {
	if !isUUID(id) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	var a sqlArgs

	sets := []string{"updated_at = now()"}
	set := func(col string, v any) { sets = append(sets, col+" = "+a.add(v)) }

	if m.LabelOverride != nil {
		set("label_override", *m.LabelOverride)
	}

	if m.Notes != nil {
		set("notes", *m.Notes)
	}

	if m.Tags != nil {
		set("tags", nonNilStrings(m.Tags))
	}

	if m.SignerPolicy != nil {
		set("signer_policy", *m.SignerPolicy)
	}

	if m.UpstreamKind != nil {
		set("upstream_kind", *m.UpstreamKind)
	}

	if m.UpstreamRef != nil {
		set("upstream_ref", *m.UpstreamRef)
	}

	if m.UpstreamLatest != nil {
		set("upstream_latest", *m.UpstreamLatest)
	}

	query := `UPDATE android_apps a SET ` + strings.Join(sets, ", ") +
		` WHERE a.repository_id = ` + a.add(repoID) + ` AND a.id = ` + a.add(id) + ` AND a.deleted_at IS NULL RETURNING ` + androidAppCols

	app, err := scanAndroidApp(p.pool.QueryRow(ctx, query, a.args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update android app: %w", err)
	}

	return app, nil
}

const androidVersionCols = `v.id::text, v.repository_id::text, v.app_id::text, v.package_name, v.version_code, v.version_name, v.file_kind,
	v.blob_id::text, v.sha256, v.size_bytes, v.filename, v.signer_sha256, v.abis, v.variant_key,
	v.label, v.labels, v.launcher_label, v.min_sdk, v.target_sdk, v.compile_sdk, v.max_sdk, v.debuggable, v.locales, v.frameworks,
	v.sig_schemes, v.sig_verified, v.content_digest, v.page_size_16k,
	v.origin, COALESCE(v.parent_id::text, ''), v.patch_info, v.source_url, v.original_sha256,
	v.changelog, v.notes, v.tags, v.channel, v.pinned, v.download_count, v.created_by, v.created_at, v.updated_at`

const androidVersionDetailCols = `, v.manifest, v.manifest_xml, v.signature, v.native_libs, v.dependencies, v.build_info, v.zip_checks, v.entries, v.warnings`

func scanAndroidVersion(row rowScanner, detail bool) (*models.AndroidVersion, error) {
	v := &models.AndroidVersion{}

	dest := scanDest(v)

	if detail {
		dest = append(dest, &v.Manifest, &v.ManifestXML, &v.Signature, &v.NativeLibs, &v.Dependencies, &v.BuildInfo,
			&v.ZipChecks, &v.Entries, &v.Warnings)
	}

	if err := row.Scan(dest...); err != nil {
		return nil, err //nolint:wrapcheck // callers wrap with their own context and test pgx.ErrNoRows
	}

	return v, nil
}

// CreateAndroidVersion inserts a version and its icons in one transaction and
// fills v.ID, v.CreatedAt and v.UpdatedAt. It returns ErrDuplicate when the
// package, versionCode, signer and ABI set are already stored.
func (p *PostgresDB) CreateAndroidVersion(ctx context.Context, v *models.AndroidVersion, icons []models.AndroidIcon) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		INSERT INTO android_versions (repository_id, app_id, package_name, version_code, version_name, file_kind,
			blob_id, sha256, size_bytes, filename, signer_sha256, abis, variant_key,
			label, labels, launcher_label, min_sdk, target_sdk, compile_sdk, max_sdk, debuggable, manifest, manifest_xml, locales, frameworks,
			sig_schemes, sig_verified, signature, content_digest, native_libs, page_size_16k, dependencies, build_info, zip_checks, entries, warnings,
			origin, parent_id, patch_info, source_url, original_sha256, changelog, notes, tags, channel, pinned, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25,
			$26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, NULLIF($38::text, '')::uuid, $39, $40, $41, $42, $43, $44, $45, $46, $47)
		RETURNING id::text, created_at, updated_at`,
		v.RepositoryID, v.AppID, v.PackageName, v.VersionCode, v.VersionName, v.FileKind,
		v.BlobID, v.SHA256, v.SizeBytes, v.Filename, v.SignerSHA256, nonNilStrings(v.ABIs), v.VariantKey,
		v.Label, rawOr(v.Labels, "{}"), v.LauncherLabel, v.MinSDK, v.TargetSDK, v.CompileSDK, v.MaxSDK, v.Debuggable,
		rawOr(v.Manifest, "{}"), v.ManifestXML, nonNilStrings(v.Locales), nonNilStrings(v.Frameworks),
		nonNilStrings(v.SigSchemes), v.SigVerified, rawOr(v.Signature, "{}"), v.ContentDigest,
		rawOr(v.NativeLibs, "[]"), v.PageSize16K, rawOr(v.Dependencies, "[]"), rawOr(v.BuildInfo, "{}"),
		rawOr(v.ZipChecks, "{}"), rawOr(v.Entries, "[]"), rawOr(v.Warnings, "[]"),
		v.Origin, v.ParentID, rawOr(v.PatchInfo, "{}"), v.SourceURL, v.OriginalSHA256, v.Changelog, v.Notes,
		nonNilStrings(v.Tags), v.Channel, v.Pinned, v.CreatedBy,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
	if IsUniqueViolation(err) {
		return ErrDuplicate
	}

	if err != nil {
		return fmt.Errorf("failed to create android version: %w", err)
	}

	for i := range icons {
		ic := &icons[i]

		if err := tx.QueryRow(ctx, `
			INSERT INTO android_icons (version_id, role, density, width, height, mime, source_path, resource_name, color,
				adaptive, renderable, blob_id, sha256, size_bytes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12::text, '')::uuid, $13, $14)
			RETURNING id::text, created_at`,
			v.ID, ic.Role, ic.Density, ic.Width, ic.Height, ic.Mime, ic.SourcePath, ic.ResourceName, ic.Color,
			ic.Adaptive, ic.Renderable, ic.BlobID, ic.SHA256, ic.SizeBytes,
		).Scan(&ic.ID, &ic.CreatedAt); err != nil {
			return fmt.Errorf("failed to create android icon: %w", err)
		}

		ic.VersionID = v.ID
	}

	if _, err := tx.Exec(ctx, `UPDATE android_apps SET updated_at = now() WHERE id = $1`, v.AppID); err != nil {
		return fmt.Errorf("failed to touch android app: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit android version: %w", err)
	}

	return nil
}

// GetAndroidVersion returns the version of the repository with the id, or nil,
// nil. detail also loads the manifest, signature and entry lists.
func (p *PostgresDB) GetAndroidVersion(ctx context.Context, repoID, id string, detail bool) (*models.AndroidVersion, error) {
	if !isUUID(id) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	cols := androidVersionCols
	if detail {
		cols += androidVersionDetailCols
	}

	v, err := scanAndroidVersion(p.pool.QueryRow(ctx,
		`SELECT `+cols+` FROM android_versions v WHERE v.repository_id = $1 AND v.id = $2 AND v.deleted_at IS NULL`,
		repoID, id), detail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android version: %w", err)
	}

	return v, nil
}

// GetAndroidVersionBySHA256 returns the newest live version of the repository
// whose file has the sha256, or nil, nil.
func (p *PostgresDB) GetAndroidVersionBySHA256(ctx context.Context, repoID, sha string) (*models.AndroidVersion, error) {
	v, err := scanAndroidVersion(p.pool.QueryRow(ctx,
		`SELECT `+androidVersionCols+` FROM android_versions v
		 WHERE v.repository_id = $1 AND v.sha256 = $2 AND v.deleted_at IS NULL ORDER BY v.created_at DESC LIMIT 1`,
		repoID, sha), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android version: %w", err)
	}

	return v, nil
}

// GetAndroidDownload resolves a download path: the version of the package with
// versionCode and the file name. Several signers can share a versionCode, so
// the file name (which includes the signer prefix) disambiguates; when it does
// not match, the newest variant is returned.
func (p *PostgresDB) GetAndroidDownload(ctx context.Context, repoID, packageName string, versionCode int64, filename string) (*models.AndroidVersion, error) {
	v, err := scanAndroidVersion(p.pool.QueryRow(ctx,
		`SELECT `+androidVersionCols+` FROM android_versions v
		 WHERE v.repository_id = $1 AND v.package_name = $2 AND v.version_code = $3 AND v.deleted_at IS NULL
		 ORDER BY (v.filename = $4) DESC, v.created_at DESC LIMIT 1`,
		repoID, packageName, versionCode, filename), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android download: %w", err)
	}

	return v, nil
}

// FindAndroidOriginal returns the stored original of a package and versionCode, or nil, nil.
func (p *PostgresDB) FindAndroidOriginal(ctx context.Context, repoID, packageName string, versionCode int64) (*models.AndroidVersion, error) {
	v, err := scanAndroidVersion(p.pool.QueryRow(ctx,
		`SELECT `+androidVersionCols+` FROM android_versions v
		 WHERE v.repository_id = $1 AND v.package_name = $2 AND v.version_code = $3 AND v.origin = 'original' AND v.deleted_at IS NULL
		 ORDER BY v.created_at DESC LIMIT 1`,
		repoID, packageName, versionCode), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find android original: %w", err)
	}

	return v, nil
}

// ListAndroidVersions returns versions newest first and the total matching.
func (p *PostgresDB) ListAndroidVersions(ctx context.Context, repoID string, f models.AndroidVersionFilter) ([]models.AndroidVersion, int, error) {
	var a sqlArgs

	where := []string{"v.repository_id = " + a.add(repoID), "v.deleted_at IS NULL"}

	if f.AppID != "" {
		where = append(where, "v.app_id = "+a.add(f.AppID))
	}

	if f.Origin != "" {
		where = append(where, "v.origin = "+a.add(f.Origin))
	}

	if f.Signer != "" {
		where = append(where, "v.signer_sha256 = "+a.add(f.Signer))
	}

	if len(f.Tags) > 0 {
		where = append(where, "v.tags @> "+a.add(f.Tags))
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `SELECT ` + androidVersionCols + `, count(*) OVER() FROM android_versions v WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY v.version_code DESC, v.created_at DESC, v.id LIMIT ` + a.add(limit) + ` OFFSET ` + a.add(max(f.Offset, 0))

	rows, err := p.pool.Query(ctx, query, a.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list android versions: %w", err)
	}
	defer rows.Close()

	var (
		out   []models.AndroidVersion
		total int
	)

	for rows.Next() {
		v := models.AndroidVersion{}

		dest := scanDest(&v)
		if err := rows.Scan(append(dest, &total)...); err != nil {
			return nil, 0, fmt.Errorf("failed to scan android version: %w", err)
		}

		out = append(out, v)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to list android versions: %w", err)
	}

	return out, total, nil
}

// scanDest lists the scan targets of androidVersionCols.
func scanDest(v *models.AndroidVersion) []any {
	return []any{&v.ID, &v.RepositoryID, &v.AppID, &v.PackageName, &v.VersionCode, &v.VersionName, &v.FileKind,
		&v.BlobID, &v.SHA256, &v.SizeBytes, &v.Filename, &v.SignerSHA256, &v.ABIs, &v.VariantKey,
		&v.Label, &v.Labels, &v.LauncherLabel, &v.MinSDK, &v.TargetSDK, &v.CompileSDK, &v.MaxSDK, &v.Debuggable, &v.Locales, &v.Frameworks,
		&v.SigSchemes, &v.SigVerified, &v.ContentDigest, &v.PageSize16K,
		&v.Origin, &v.ParentID, &v.PatchInfo, &v.SourceURL, &v.OriginalSHA256,
		&v.Changelog, &v.Notes, &v.Tags, &v.Channel, &v.Pinned, &v.DownloadCount, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt}
}

// AndroidLatestFilter selects the version latest.apk / latest.json serve.
type AndroidLatestFilter struct {
	SDK     int    // device API level: min_sdk must not exceed it
	ABI     string // device ABI: universal files and files listing it match
	Channel string
	Signer  string
	Origin  string
}

// LatestAndroidVersion returns the highest versionCode of the package that
// matches the filter, or nil, nil.
func (p *PostgresDB) LatestAndroidVersion(ctx context.Context, repoID, packageName string, f AndroidLatestFilter) (*models.AndroidVersion, error) {
	var a sqlArgs

	where := []string{"v.repository_id = " + a.add(repoID), "v.package_name = " + a.add(packageName), "v.deleted_at IS NULL"}

	if f.SDK > 0 {
		where = append(where, "v.min_sdk <= "+a.add(f.SDK))
	}

	if f.ABI != "" {
		where = append(where, "(cardinality(v.abis) = 0 OR "+a.add(f.ABI)+" = ANY(v.abis))")
	}

	if f.Channel != "" {
		where = append(where, "v.channel = "+a.add(f.Channel))
	}

	if f.Signer != "" {
		where = append(where, "v.signer_sha256 = "+a.add(f.Signer))
	}

	if f.Origin != "" {
		where = append(where, "v.origin = "+a.add(f.Origin))
	}

	v, err := scanAndroidVersion(p.pool.QueryRow(ctx,
		`SELECT `+androidVersionCols+` FROM android_versions v WHERE `+strings.Join(where, " AND ")+
			` ORDER BY v.version_code DESC, v.created_at DESC LIMIT 1`, a.args...), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get latest android version: %w", err)
	}

	return v, nil
}

// UpdateAndroidVersionMeta applies the non-nil fields of m and returns the
// version, or nil, nil when it does not exist in the repository. A parent from
// another repository is rejected with ErrAndroidBadParent.
func (p *PostgresDB) UpdateAndroidVersionMeta(ctx context.Context, repoID, id string, m models.AndroidVersionMeta) (*models.AndroidVersion, error) {
	if !isUUID(id) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	var a sqlArgs

	sets := []string{"updated_at = now()"}
	set := func(col string, v any) { sets = append(sets, col+" = "+a.add(v)) }

	if m.Notes != nil {
		set("notes", *m.Notes)
	}

	if m.Changelog != nil {
		set("changelog", *m.Changelog)
	}

	if m.Tags != nil {
		set("tags", nonNilStrings(m.Tags))
	}

	if m.Channel != nil {
		set("channel", *m.Channel)
	}

	if m.Origin != nil {
		set("origin", *m.Origin)
	}

	if m.ParentID != nil {
		if *m.ParentID != "" && *m.ParentID == id {
			return nil, ErrAndroidBadParent
		}

		if *m.ParentID == "" {
			sets = append(sets, "parent_id = NULL")
		} else {
			// Only a live version of the same repository can be a parent.
			sets = append(sets, "parent_id = (SELECT p.id FROM android_versions p WHERE p.id = "+a.add(*m.ParentID)+
				"::uuid AND p.repository_id = v.repository_id AND p.deleted_at IS NULL)")
		}
	}

	if m.SourceURL != nil {
		set("source_url", *m.SourceURL)
	}

	if len(m.PatchInfo) > 0 {
		set("patch_info", []byte(m.PatchInfo))
	}

	if m.Pinned != nil {
		set("pinned", *m.Pinned)
	}

	query := `UPDATE android_versions v SET ` + strings.Join(sets, ", ") +
		` WHERE v.repository_id = ` + a.add(repoID) + ` AND v.id = ` + a.add(id) + ` AND v.deleted_at IS NULL RETURNING ` + androidVersionCols

	v, err := scanAndroidVersion(p.pool.QueryRow(ctx, query, a.args...), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update android version: %w", err)
	}

	if m.ParentID != nil && *m.ParentID != "" && v.ParentID == "" {
		return nil, ErrAndroidBadParent
	}

	return v, nil
}

// ErrAndroidBadParent is returned when a parent version does not exist in the repository.
var ErrAndroidBadParent = errors.New("parent version not found in this repository")

// AndroidDeleted describes what DeleteAndroidVersion removed: the blobs the
// caller must release with storage.SafeDeleteBlob and the quota to give back.
type AndroidDeleted struct {
	Version    *models.AndroidVersion
	BlobSHA256 []string
	QuotaBytes int64
}

// DeleteAndroidVersion removes a version with its icons and attached files.
// It returns nil, nil when the version is not in the repository.
func (p *PostgresDB) DeleteAndroidVersion(ctx context.Context, repoID, id string) (*AndroidDeleted, error) {
	v, err := p.GetAndroidVersion(ctx, repoID, id, false)
	if err != nil || v == nil {
		return nil, err
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	out := &AndroidDeleted{Version: v, BlobSHA256: []string{v.SHA256}, QuotaBytes: v.SizeBytes}
	seen := map[string]bool{v.SHA256: true}

	rows, err := tx.Query(ctx, `
		SELECT sha256, size_bytes, true FROM android_files WHERE version_id = $1
		UNION ALL SELECT sha256, 0, false FROM android_icons WHERE version_id = $1 AND blob_id IS NOT NULL`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to list android blobs: %w", err)
	}

	for rows.Next() {
		var (
			sha    string
			size   int64
			quoted bool
		)

		if err := rows.Scan(&sha, &size, &quoted); err != nil {
			rows.Close()

			return nil, fmt.Errorf("failed to scan android blob: %w", err)
		}

		if !seen[sha] {
			seen[sha] = true
			out.BlobSHA256 = append(out.BlobSHA256, sha)
		}

		if quoted {
			out.QuotaBytes += size
		}
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list android blobs: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM android_versions WHERE id = $1 AND repository_id = $2`, id, repoID); err != nil {
		return nil, fmt.Errorf("failed to delete android version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit android delete: %w", err)
	}

	return out, nil
}

// IncrementAndroidDownload counts one download of a version.
func (p *PostgresDB) IncrementAndroidDownload(ctx context.Context, id string) error {
	if _, err := p.pool.Exec(ctx, `UPDATE android_versions SET download_count = download_count + 1 WHERE id = $1`, id); err != nil {
		return fmt.Errorf("failed to count android download: %w", err)
	}

	return nil
}
