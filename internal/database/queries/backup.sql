-- name: CreateBackupJob :one
INSERT INTO backup_jobs (status, type, storage_path, size_bytes, error_message, started_at, completed_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: GetBackupJob :one
SELECT id, status, type,
       COALESCE(storage_path, '') AS storage_path,
       COALESCE(size_bytes, 0) AS size_bytes,
       COALESCE(error_message, '') AS error_message,
       started_at, completed_at, created_at, updated_at
FROM backup_jobs WHERE id = $1;

-- name: UpdateBackupJob :exec
UPDATE backup_jobs
SET status = $1, type = $2, storage_path = $3, size_bytes = $4, error_message = $5, started_at = $6, completed_at = $7, updated_at = $8
WHERE id = $9;

-- name: ListBackupJobs :many
SELECT id, status, type,
       COALESCE(storage_path, '') AS storage_path,
       COALESCE(size_bytes, 0) AS size_bytes,
       COALESCE(error_message, '') AS error_message,
       started_at, completed_at, created_at, updated_at
FROM backup_jobs ORDER BY created_at DESC LIMIT $1;
