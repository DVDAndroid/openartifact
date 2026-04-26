-- name: GetSystemSetting :one
SELECT value FROM system_settings WHERE key = $1;

-- name: SetSystemSetting :exec
INSERT INTO system_settings (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now();
