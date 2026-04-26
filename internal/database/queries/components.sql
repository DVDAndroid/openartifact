-- Debian package queries against deb_packages. Package data lives in the
-- per-format rpm_packages and deb_packages tables; their blob_id FKs point
-- into the global blobs table, which carries the sha256/size columns.
--
-- RPM-side lookups (GetComponentByID, GetComponentByFilename,
-- GetComponentsByRepo, DeleteComponent) are hand-written in postgres.go
-- because they must try both rpm_packages and deb_packages (IDs are shared
-- across the historical split) — no sqlc query is generated for them.

-- name: GetDebianPackageByFilename :one
SELECT dp.id, dp.repository_id, dp.distribution_id, dp.component_id,
       COALESCE(b.sha256, '') AS sha256,
       dp.name, dp.version, dp.arch,
       COALESCE(dp.summary, '') AS summary,
       COALESCE(dp.description, '') AS description,
       COALESCE(dp.license, '') AS license,
       COALESCE(dp.maintainer, '') AS maintainer,
       COALESCE(dp.homepage, '') AS homepage,
       COALESCE(dp.filename, '') AS filename,
       COALESCE(b.size_bytes, 0) AS package_size,
       COALESCE(dp.installed_size, 0) AS installed_size,
       dp.created_at, dp.updated_at
FROM deb_packages dp
LEFT JOIN blobs b ON b.id = dp.blob_id
WHERE dp.repository_id = $1 AND dp.filename = $2;

-- name: GetDebianPackagesByDistComp :many
SELECT dp.id, dp.repository_id, dp.distribution_id, dp.component_id,
       COALESCE(b.sha256, '') AS sha256,
       dp.name, dp.version, dp.arch,
       COALESCE(dp.summary, '') AS summary,
       COALESCE(dp.description, '') AS description,
       COALESCE(dp.license, '') AS license,
       COALESCE(dp.maintainer, '') AS maintainer,
       COALESCE(dp.homepage, '') AS homepage,
       COALESCE(dp.filename, '') AS filename,
       COALESCE(b.size_bytes, 0) AS package_size,
       COALESCE(dp.installed_size, 0) AS installed_size,
       dp.created_at, dp.updated_at
FROM deb_packages dp
LEFT JOIN blobs b ON b.id = dp.blob_id
JOIN debian_distributions dd ON dd.id = dp.distribution_id
JOIN debian_components dc ON dc.id = dp.component_id
WHERE dp.repository_id = $1
  AND (sqlc.arg(dist)::text = '' OR dd.name = sqlc.arg(dist))
  AND (sqlc.arg(comp)::text = '' OR dc.name = sqlc.arg(comp))
  AND (sqlc.arg(arch)::text = '' OR dp.arch = sqlc.arg(arch));
