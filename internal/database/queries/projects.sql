-- name: CreateProject :one
INSERT INTO projects (id, name, slug, display_name, description, owner_id,
                      allowed_formats, allowed_repo_types, repo_name_prefix, allow_public_repos, max_repositories, token_max_ttl_days)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, created_at, updated_at;

-- name: GetProject :one
SELECT *
FROM projects WHERE id = $1;

-- name: GetProjectByName :one
SELECT *
FROM projects WHERE name = $1;

-- name: UpdateProject :exec
UPDATE projects SET display_name = $1, description = $2, owner_id = $3,
       allowed_formats = $4, allowed_repo_types = $5, repo_name_prefix = $6, allow_public_repos = $7,
       max_repositories = $8, token_max_ttl_days = $9, updated_at = now()
WHERE id = $10;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: ListProjectsByUser :many
-- Projects the user belongs to: recorded owner, direct member or through a group.
SELECT p.*
FROM projects p
WHERE p.owner_id = $1
   OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = $1)
   OR EXISTS (SELECT 1 FROM project_groups pg JOIN auth_user_groups ug ON ug.group_id = pg.group_id
              WHERE pg.project_id = p.id AND ug.user_id = $1)
ORDER BY p.created_at DESC;

-- name: ListAllProjects :many
SELECT *
FROM projects ORDER BY created_at DESC, id;

-- name: AddProjectMember :one
INSERT INTO project_members (project_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING created_at;

-- name: RemoveProjectMember :exec
DELETE FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: ListProjectMembers :many
SELECT (project_id::text || ':' || user_id::text) AS id, project_id, user_id, role, created_at
FROM project_members WHERE project_id = $1 ORDER BY created_at ASC;

-- name: GetProjectMember :one
SELECT (project_id::text || ':' || user_id::text) AS id, project_id, user_id, role, created_at
FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: UpdateProjectMemberRole :exec
UPDATE project_members SET role = $1 WHERE project_id = $2 AND user_id = $3;

-- name: ListProjectGroups :many
SELECT pg.project_id, pg.group_id, g.name AS group_name, pg.role, pg.created_by, pg.created_at
FROM project_groups pg JOIN auth_groups g ON g.id = pg.group_id
WHERE pg.project_id = $1 ORDER BY g.name ASC;

-- name: GetProjectGroup :one
SELECT pg.project_id, pg.group_id, g.name AS group_name, pg.role, pg.created_by, pg.created_at
FROM project_groups pg JOIN auth_groups g ON g.id = pg.group_id
WHERE pg.project_id = $1 AND pg.group_id = $2;

-- name: SetProjectGroup :exec
INSERT INTO project_groups (project_id, group_id, role, created_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (project_id, group_id) DO UPDATE SET role = EXCLUDED.role;

-- name: RemoveProjectGroup :exec
DELETE FROM project_groups WHERE project_id = $1 AND group_id = $2;

-- name: ListProjectGrantsByUser :many
-- Every project role the user holds, one row per source: the recorded owner
-- (always owner), a membership row, or a group attached to the project.
-- The caller keeps the highest role per project.
SELECT p.id AS project_id, 'owner'::text AS role
FROM projects p WHERE p.owner_id = $1
UNION ALL
SELECT pm.project_id, pm.role::text
FROM project_members pm WHERE pm.user_id = $1
UNION ALL
SELECT pg.project_id, pg.role::text
FROM project_groups pg JOIN auth_user_groups ug ON ug.group_id = pg.group_id
WHERE ug.user_id = $1;
