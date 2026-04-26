-- name: CreateDockerBlob :one
INSERT INTO docker_blobs (repository_id, digest, blob_id)
VALUES ($1, $2, $3)
RETURNING id, created_at;

-- name: GetDockerBlob :one
SELECT dbb.id, dbb.repository_id, dbb.digest, b.media_type, b.size_bytes AS size, b.sha256 AS blob_sha256, dbb.created_at
FROM docker_blobs dbb
JOIN blobs b ON b.id = dbb.blob_id
WHERE dbb.repository_id = $1 AND dbb.digest = $2;

-- name: DeleteDockerBlob :exec
DELETE FROM docker_blobs WHERE repository_id = $1 AND digest = $2;

-- name: ListDockerBlobs :many
SELECT dbb.id, dbb.repository_id, dbb.digest, b.media_type, b.size_bytes AS size, b.sha256 AS blob_sha256, dbb.created_at
FROM docker_blobs dbb
JOIN blobs b ON b.id = dbb.blob_id
WHERE dbb.repository_id = $1
ORDER BY dbb.created_at DESC;

-- name: CreateDockerUpload :one
INSERT INTO docker_uploads (repository_id, upload_uuid, "offset")
VALUES ($1, $2, $3)
RETURNING id, created_at, updated_at;

-- name: GetDockerUpload :one
SELECT id, repository_id, upload_uuid, "offset", created_at, updated_at
FROM docker_uploads
WHERE repository_id = $1 AND upload_uuid = $2;

-- name: UpdateDockerUploadOffset :exec
UPDATE docker_uploads SET "offset" = $1, updated_at = now() WHERE repository_id = $2 AND upload_uuid = $3;

-- name: DeleteDockerUpload :exec
DELETE FROM docker_uploads WHERE repository_id = $1 AND upload_uuid = $2;

-- name: CreateDockerManifest :one
INSERT INTO docker_manifests (repository_id, digest, media_type, blob_id, size, subject_digest)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
RETURNING id, created_at, updated_at;

-- name: GetDockerManifest :one
SELECT id, repository_id, digest, media_type, blob_id, size,
       COALESCE(subject_digest, '') AS subject_digest,
       created_at, updated_at
FROM docker_manifests
WHERE repository_id = $1 AND digest = $2;

-- name: DeleteDockerManifest :exec
DELETE FROM docker_manifests WHERE repository_id = $1 AND digest = $2;

-- name: UpsertDockerTag :one
INSERT INTO docker_tags (repository_id, name, manifest_digest)
VALUES ($1, $2, $3)
ON CONFLICT (repository_id, name) DO UPDATE SET manifest_digest = EXCLUDED.manifest_digest, updated_at = now()
RETURNING id, created_at, updated_at;

-- name: GetDockerTag :one
SELECT id, repository_id, name, manifest_digest, created_at, updated_at
FROM docker_tags
WHERE repository_id = $1 AND name = $2;

-- name: DeleteDockerTag :exec
DELETE FROM docker_tags WHERE repository_id = $1 AND name = $2;

-- name: ListDockerTags :many
SELECT id, repository_id, name, manifest_digest, created_at, updated_at
FROM docker_tags
WHERE repository_id = $1
ORDER BY name ASC
LIMIT $2;

-- name: ListDockerTagsAfter :many
SELECT id, repository_id, name, manifest_digest, created_at, updated_at
FROM docker_tags
WHERE repository_id = $1 AND name > $2
ORDER BY name ASC
LIMIT $3;

-- name: ListDockerManifestsByRepo :many
SELECT dm.id, dm.repository_id, dm.digest, dm.media_type, b.sha256 AS blob_sha256, dm.size,
       COALESCE(dm.subject_digest, '') AS subject_digest,
       dm.created_at, dm.updated_at
FROM docker_manifests dm
JOIN blobs b ON b.id = dm.blob_id
WHERE dm.repository_id = $1
ORDER BY dm.created_at;
