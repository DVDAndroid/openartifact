-- name: InsertAuditLog :exec
INSERT INTO audit_logs (action, actor_id, actor_type, details_json, resource_type, resource_id, repository_id, org_id, ip_address)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
