-- name: CreateWebhook :one
INSERT INTO webhooks (repository_id, name, url, secret_enc, enabled, events, content_type, ssl_verify, max_retries, timeout_seconds)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, created_at, updated_at;

-- name: GetWebhook :one
SELECT id, COALESCE(repository_id::text, '') AS repository_id, name, url, COALESCE(secret_enc, '') AS secret_enc, enabled, COALESCE(events, '{}') AS events, content_type, ssl_verify, max_retries, timeout_seconds, created_at, updated_at
FROM webhooks WHERE id = $1;

-- name: UpdateWebhook :exec
UPDATE webhooks
SET name = $2, url = $3, secret_enc = $4, enabled = $5, events = $6, content_type = $7, ssl_verify = $8, max_retries = $9, timeout_seconds = $10, updated_at = now()
WHERE id = $1;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = $1;

-- name: ListGlobalWebhooks :many
SELECT id, COALESCE(repository_id::text, '') AS repository_id, name, url, COALESCE(secret_enc, '') AS secret_enc, enabled, COALESCE(events, '{}') AS events, content_type, ssl_verify, max_retries, timeout_seconds, created_at, updated_at
FROM webhooks WHERE repository_id IS NULL ORDER BY created_at DESC;

-- name: ListWebhooksByRepo :many
SELECT id, COALESCE(repository_id::text, '') AS repository_id, name, url, COALESCE(secret_enc, '') AS secret_enc, enabled, COALESCE(events, '{}') AS events, content_type, ssl_verify, max_retries, timeout_seconds, created_at, updated_at
FROM webhooks WHERE repository_id = $1 ORDER BY created_at DESC;

-- name: ListWebhooksForEvent :many
SELECT id, COALESCE(repository_id::text, '') AS repository_id, name, url, COALESCE(secret_enc, '') AS secret_enc, enabled, COALESCE(events, '{}') AS events, content_type, ssl_verify, max_retries, timeout_seconds, created_at, updated_at
FROM webhooks
WHERE (repository_id = $1 OR repository_id IS NULL) AND enabled = true AND $2 = ANY(events)
ORDER BY created_at DESC;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status, attempt_count)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at;

-- name: UpdateWebhookDelivery :exec
UPDATE webhook_deliveries
SET status = $2, response_status = $3, response_body = $4, attempt_count = $5, next_retry_at = $6, delivered_at = $7, payload = $8, updated_at = now()
WHERE id = $1;

-- name: ListWebhookDeliveries :many
SELECT id, webhook_id, event_type, payload, status, response_status, response_body, attempt_count, next_retry_at, delivered_at, created_at, updated_at
FROM webhook_deliveries WHERE webhook_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListPendingWebhookDeliveries :many
SELECT id, webhook_id, event_type, payload, status, response_status, response_body, attempt_count, next_retry_at, delivered_at, created_at, updated_at
FROM webhook_deliveries
WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY created_at ASC LIMIT 100;
