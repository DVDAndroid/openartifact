-- name: CreateOrganization :one
INSERT INTO organizations (id, name, display_name, description, owner_id)
VALUES (gen_random_uuid(), $1, $2, $3, $4)
RETURNING id, created_at, updated_at;

-- name: GetOrganization :one
SELECT id, name, display_name, description, owner_id, created_at, updated_at
FROM organizations WHERE id = $1;

-- name: GetOrganizationByName :one
SELECT id, name, display_name, description, owner_id, created_at, updated_at
FROM organizations WHERE name = $1;

-- name: UpdateOrganization :exec
UPDATE organizations SET display_name = $1, description = $2, owner_id = $3, updated_at = now() WHERE id = $4;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = $1;

-- name: ListOrganizationsByUser :many
SELECT DISTINCT o.id, o.name, o.display_name, o.description, o.owner_id, o.created_at, o.updated_at
FROM organizations o
LEFT JOIN organization_members om ON o.id = om.org_id
WHERE o.owner_id = $1 OR om.user_id = $1
ORDER BY o.created_at DESC;

-- name: AddOrgMember :one
INSERT INTO organization_members (org_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING created_at;

-- name: RemoveOrgMember :exec
DELETE FROM organization_members WHERE org_id = $1 AND user_id = $2;

-- name: ListOrgMembers :many
SELECT (org_id::text || ':' || user_id::text) AS id, org_id, user_id, role, created_at
FROM organization_members WHERE org_id = $1 ORDER BY created_at ASC;

-- name: GetOrgMember :one
SELECT (org_id::text || ':' || user_id::text) AS id, org_id, user_id, role, created_at
FROM organization_members WHERE org_id = $1 AND user_id = $2;

-- name: UpdateOrgMemberRole :exec
UPDATE organization_members SET role = $1 WHERE org_id = $2 AND user_id = $3;
