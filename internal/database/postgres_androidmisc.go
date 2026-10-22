// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/openartifact/openartifact/internal/database/models"
)

const androidIconCols = `i.id::text, i.version_id::text, i.role, i.density, i.width, i.height, i.mime, i.source_path, i.resource_name, i.color,
	i.adaptive, i.renderable, COALESCE(i.blob_id::text, ''), i.sha256, i.size_bytes, i.created_at`

func scanAndroidIcon(row rowScanner) (*models.AndroidIcon, error) {
	i := &models.AndroidIcon{}

	if err := row.Scan(&i.ID, &i.VersionID, &i.Role, &i.Density, &i.Width, &i.Height, &i.Mime, &i.SourcePath, &i.ResourceName,
		&i.Color, &i.Adaptive, &i.Renderable, &i.BlobID, &i.SHA256, &i.SizeBytes, &i.CreatedAt); err != nil {
		return nil, err //nolint:wrapcheck // callers wrap with their own context and test pgx.ErrNoRows
	}

	return i, nil
}

// ListAndroidIcons returns the icons of a version of the repository, largest first.
func (p *PostgresDB) ListAndroidIcons(ctx context.Context, repoID, versionID string) ([]models.AndroidIcon, error) {
	if !isUUID(versionID) {
		return nil, nil
	}

	rows, err := p.pool.Query(ctx, `SELECT `+androidIconCols+` FROM android_icons i
		JOIN android_versions v ON v.id = i.version_id
		WHERE v.repository_id = $1 AND v.id = $2
		ORDER BY i.role, i.width DESC, i.density, i.source_path`, repoID, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list android icons: %w", err)
	}
	defer rows.Close()

	var out []models.AndroidIcon

	for rows.Next() {
		i, err := scanAndroidIcon(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan android icon: %w", err)
		}

		out = append(out, *i)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list android icons: %w", err)
	}

	return out, nil
}

// GetAndroidIcon returns one icon of a version of the repository, or nil, nil.
func (p *PostgresDB) GetAndroidIcon(ctx context.Context, repoID, versionID, iconID string) (*models.AndroidIcon, error) {
	if !isUUID(versionID) || !isUUID(iconID) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	i, err := scanAndroidIcon(p.pool.QueryRow(ctx, `SELECT `+androidIconCols+` FROM android_icons i
		JOIN android_versions v ON v.id = i.version_id
		WHERE v.repository_id = $1 AND v.id = $2 AND i.id = $3`, repoID, versionID, iconID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android icon: %w", err)
	}

	return i, nil
}

const androidFileCols = `f.id::text, f.version_id::text, f.kind, f.name, f.blob_id::text, f.sha256, f.size_bytes, f.created_at`

func scanAndroidFile(row rowScanner) (*models.AndroidFile, error) {
	f := &models.AndroidFile{}

	if err := row.Scan(&f.ID, &f.VersionID, &f.Kind, &f.Name, &f.BlobID, &f.SHA256, &f.SizeBytes, &f.CreatedAt); err != nil {
		return nil, err //nolint:wrapcheck // callers wrap with their own context and test pgx.ErrNoRows
	}

	return f, nil
}

// CreateAndroidFile attaches a file to a version of the repository and fills
// f.ID and f.CreatedAt. It returns pgx.ErrNoRows wrapped when the version is
// not in the repository.
func (p *PostgresDB) CreateAndroidFile(ctx context.Context, repoID string, f *models.AndroidFile) error {
	err := p.pool.QueryRow(ctx, `
		INSERT INTO android_files (version_id, kind, name, blob_id, sha256, size_bytes)
		SELECT v.id, $3, $4, $5, $6, $7 FROM android_versions v
		WHERE v.id = $2 AND v.repository_id = $1
		RETURNING id::text, created_at`,
		repoID, f.VersionID, f.Kind, f.Name, f.BlobID, f.SHA256, f.SizeBytes).Scan(&f.ID, &f.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create android file: %w", err)
	}

	return nil
}

// ListAndroidFiles returns the files attached to a version of the repository.
func (p *PostgresDB) ListAndroidFiles(ctx context.Context, repoID, versionID string) ([]models.AndroidFile, error) {
	if !isUUID(versionID) {
		return nil, nil
	}

	rows, err := p.pool.Query(ctx, `SELECT `+androidFileCols+` FROM android_files f
		JOIN android_versions v ON v.id = f.version_id
		WHERE v.repository_id = $1 AND v.id = $2 ORDER BY f.created_at, f.id`, repoID, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list android files: %w", err)
	}
	defer rows.Close()

	var out []models.AndroidFile

	for rows.Next() {
		f, err := scanAndroidFile(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan android file: %w", err)
		}

		out = append(out, *f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list android files: %w", err)
	}

	return out, nil
}

// GetAndroidFile returns one attached file of a version of the repository, or nil, nil.
func (p *PostgresDB) GetAndroidFile(ctx context.Context, repoID, versionID, fileID string) (*models.AndroidFile, error) {
	if !isUUID(versionID) || !isUUID(fileID) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	f, err := scanAndroidFile(p.pool.QueryRow(ctx, `SELECT `+androidFileCols+` FROM android_files f
		JOIN android_versions v ON v.id = f.version_id
		WHERE v.repository_id = $1 AND v.id = $2 AND f.id = $3`, repoID, versionID, fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get android file: %w", err)
	}

	return f, nil
}

// DeleteAndroidFile removes an attached file and returns it, or nil, nil when
// it is not attached to a version of the repository.
func (p *PostgresDB) DeleteAndroidFile(ctx context.Context, repoID, versionID, fileID string) (*models.AndroidFile, error) {
	f, err := p.GetAndroidFile(ctx, repoID, versionID, fileID)
	if err != nil || f == nil {
		return nil, err
	}

	if _, err := p.pool.Exec(ctx, `DELETE FROM android_files WHERE id = $1 AND version_id = $2`, fileID, versionID); err != nil {
		return nil, fmt.Errorf("failed to delete android file: %w", err)
	}

	return f, nil
}

const androidSignerCols = `s.id::text, s.repository_id::text, s.sha256, s.label, s.trust, COALESCE(s.app_id::text, ''), s.notes, s.certificate,
	s.created_at, s.updated_at`

func scanAndroidSigner(row rowScanner) (*models.AndroidSigner, error) {
	s := &models.AndroidSigner{}

	if err := row.Scan(&s.ID, &s.RepositoryID, &s.SHA256, &s.Label, &s.Trust, &s.AppID, &s.Notes, &s.Certificate,
		&s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err //nolint:wrapcheck // callers wrap with their own context and test pgx.ErrNoRows
	}

	return s, nil
}

// ListAndroidSigners returns the known keys of a repository, optionally only
// those registered for appID (plus the repository-wide ones).
func (p *PostgresDB) ListAndroidSigners(ctx context.Context, repoID, appID string) ([]models.AndroidSigner, error) {
	var a sqlArgs

	where := "s.repository_id = " + a.add(repoID)
	if appID != "" {
		where += " AND (s.app_id IS NULL OR s.app_id = " + a.add(appID) + ")"
	}

	rows, err := p.pool.Query(ctx, `SELECT `+androidSignerCols+` FROM android_signers s WHERE `+where+` ORDER BY s.created_at, s.id`, a.args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list android signers: %w", err)
	}
	defer rows.Close()

	var out []models.AndroidSigner

	for rows.Next() {
		s, err := scanAndroidSigner(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan android signer: %w", err)
		}

		out = append(out, *s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list android signers: %w", err)
	}

	return out, nil
}

// UpsertAndroidSigner records a known key. A key already known for the same
// repository and app keeps its row and takes the new label, trust and notes;
// an empty label or notes never overwrites a stored one.
func (p *PostgresDB) UpsertAndroidSigner(ctx context.Context, s *models.AndroidSigner) error {
	row, err := scanAndroidSigner(p.pool.QueryRow(ctx, `
		INSERT INTO android_signers AS s (repository_id, sha256, label, trust, app_id, notes, certificate)
		VALUES ($1, $2, $3, $4, NULLIF($5::text, '')::uuid, $6, $7)
		ON CONFLICT (repository_id, sha256, coalesce(app_id, '00000000-0000-0000-0000-000000000000'::uuid)) DO UPDATE SET
			trust = EXCLUDED.trust,
			label = CASE WHEN EXCLUDED.label = '' THEN s.label ELSE EXCLUDED.label END,
			notes = CASE WHEN EXCLUDED.notes = '' THEN s.notes ELSE EXCLUDED.notes END,
			certificate = COALESCE(EXCLUDED.certificate, s.certificate),
			updated_at = now()
		RETURNING `+androidSignerCols,
		s.RepositoryID, s.SHA256, s.Label, s.Trust, s.AppID, s.Notes, nullableJSON(s.Certificate)))
	if err != nil {
		return fmt.Errorf("failed to upsert android signer: %w", err)
	}

	*s = *row

	return nil
}

// UpdateAndroidSigner changes label, trust and notes of a known key, or returns nil, nil.
func (p *PostgresDB) UpdateAndroidSigner(ctx context.Context, repoID, id, label, trust, notes string) (*models.AndroidSigner, error) {
	if !isUUID(id) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	s, err := scanAndroidSigner(p.pool.QueryRow(ctx, `
		UPDATE android_signers s SET label = $3, trust = $4, notes = $5, updated_at = now()
		WHERE s.repository_id = $1 AND s.id = $2 RETURNING `+androidSignerCols, repoID, id, label, trust, notes))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil, nil is the standard Go "not found" pattern
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update android signer: %w", err)
	}

	return s, nil
}

// DeleteAndroidSigner forgets a known key and reports whether it existed.
func (p *PostgresDB) DeleteAndroidSigner(ctx context.Context, repoID, id string) (bool, error) {
	if !isUUID(id) {
		return false, nil
	}

	tag, err := p.pool.Exec(ctx, `DELETE FROM android_signers WHERE repository_id = $1 AND id = $2`, repoID, id)
	if err != nil {
		return false, fmt.Errorf("failed to delete android signer: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// AndroidSignerTrust returns how a key is known for an app: a registration
// scoped to the app wins over a repository-wide one. found is false for an
// unknown key.
func (p *PostgresDB) AndroidSignerTrust(ctx context.Context, repoID, appID, sha string) (trust string, found bool, err error) {
	err = p.pool.QueryRow(ctx, `
		SELECT trust FROM android_signers
		WHERE repository_id = $1 AND sha256 = $3 AND (app_id IS NULL OR app_id = $2)
		ORDER BY (app_id IS NOT NULL) DESC LIMIT 1`, repoID, appID, sha).Scan(&trust)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("failed to look up android signer: %w", err)
	}

	return trust, true, nil
}

// ListAndroidAppSignerUse returns the keys the versions of an app are signed with.
func (p *PostgresDB) ListAndroidAppSignerUse(ctx context.Context, repoID, appID string) ([]models.AndroidSignerUse, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT v.signer_sha256, count(*)::int, min(v.created_at), max(v.created_at), array_agg(DISTINCT v.origin)
		FROM android_versions v
		WHERE v.repository_id = $1 AND v.app_id = $2
		GROUP BY v.signer_sha256 ORDER BY max(v.created_at) DESC`, repoID, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to list android signer use: %w", err)
	}
	defer rows.Close()

	var out []models.AndroidSignerUse

	for rows.Next() {
		var u models.AndroidSignerUse

		if err := rows.Scan(&u.SHA256, &u.Versions, &u.First, &u.Last, &u.Origins); err != nil {
			return nil, fmt.Errorf("failed to scan android signer use: %w", err)
		}

		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list android signer use: %w", err)
	}

	return out, nil
}

// ListAndroidTags returns the tags in use in a repository with their counts.
func (p *PostgresDB) ListAndroidTags(ctx context.Context, repoID string) ([]models.AndroidTagCount, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT tag, sum(apps)::int, sum(versions)::int FROM (
			SELECT unnest(tags) AS tag, 1 AS apps, 0 AS versions FROM android_apps WHERE repository_id = $1
			UNION ALL
			SELECT unnest(tags), 0, 1 FROM android_versions WHERE repository_id = $1
		) t GROUP BY tag ORDER BY sum(apps) + sum(versions) DESC, tag`, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list android tags: %w", err)
	}
	defer rows.Close()

	var out []models.AndroidTagCount

	for rows.Next() {
		var t models.AndroidTagCount

		if err := rows.Scan(&t.Tag, &t.Apps, &t.Versions); err != nil {
			return nil, fmt.Errorf("failed to scan android tag: %w", err)
		}

		out = append(out, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list android tags: %w", err)
	}

	return out, nil
}
