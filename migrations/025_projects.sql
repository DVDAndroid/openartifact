-- SPDX-FileCopyrightText: The OpenArtifact Authors
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Organizations become projects.
--
-- A project owns repositories; its members (users directly, groups through
-- project_groups) hold a role (viewer, developer, maintainer, owner) on them.
-- The tables, columns, constraints and indexes are renamed in one step, so
-- this release must replace the previous one entirely (Recreate): a replica
-- still running the old code would query tables that no longer exist.
--
-- The built-in system project keeps its id (00000000-0000-0000-0000-000000000001):
-- it is the "unassigned" project and grants nothing.
SET LOCAL lock_timeout = '10s';

-- Renames ---------------------------------------------------------------------

ALTER TABLE organizations RENAME TO projects;
ALTER TABLE organization_members RENAME TO project_members;

ALTER TABLE project_members RENAME COLUMN org_id TO project_id;
ALTER TABLE auth_groups RENAME COLUMN org_id TO project_id;
ALTER TABLE permission_targets RENAME COLUMN org_id TO project_id;
ALTER TABLE repositories RENAME COLUMN org_id TO project_id;
ALTER TABLE audit_logs RENAME COLUMN org_id TO project_id;

ALTER TABLE projects RENAME CONSTRAINT organizations_pkey TO projects_pkey;
ALTER TABLE projects RENAME CONSTRAINT organizations_name_key TO projects_name_key;
ALTER TABLE projects RENAME CONSTRAINT organizations_slug_unique TO projects_slug_unique;
ALTER TABLE projects RENAME CONSTRAINT organizations_owner_id_fkey TO projects_owner_id_fkey;
ALTER TABLE project_members RENAME CONSTRAINT organization_members_pkey TO project_members_pkey;
ALTER TABLE project_members RENAME CONSTRAINT organization_members_org_id_fkey TO project_members_project_id_fkey;
ALTER TABLE project_members RENAME CONSTRAINT organization_members_user_id_fkey TO project_members_user_id_fkey;
ALTER TABLE auth_groups RENAME CONSTRAINT auth_groups_org_name_unique TO auth_groups_project_name_unique;
ALTER TABLE auth_groups RENAME CONSTRAINT auth_groups_org_id_fkey TO auth_groups_project_id_fkey;
ALTER TABLE permission_targets RENAME CONSTRAINT permission_targets_org_name_unique TO permission_targets_project_name_unique;
ALTER TABLE permission_targets RENAME CONSTRAINT permission_targets_org_id_fkey TO permission_targets_project_id_fkey;
ALTER TABLE repositories RENAME CONSTRAINT repositories_org_name_unique TO repositories_project_name_unique;
ALTER TABLE repositories RENAME CONSTRAINT fk_repositories_org TO fk_repositories_project;
ALTER TABLE audit_logs RENAME CONSTRAINT audit_logs_org_id_fkey TO audit_logs_project_id_fkey;

ALTER INDEX idx_org_members_org_id RENAME TO idx_project_members_project_id;
ALTER INDEX idx_org_members_user_id RENAME TO idx_project_members_user_id;
ALTER INDEX idx_auth_groups_org_id RENAME TO idx_auth_groups_project_id;
ALTER INDEX idx_permission_targets_org_id RENAME TO idx_permission_targets_project_id;
ALTER INDEX idx_repositories_org_id RENAME TO idx_repositories_project_id;
ALTER INDEX idx_audit_logs_org_id RENAME TO idx_audit_logs_project_id;

ALTER TRIGGER update_organizations_updated_at ON projects RENAME TO update_projects_updated_at;
ALTER TRIGGER update_organization_members_updated_at ON project_members RENAME TO update_project_members_updated_at;

COMMENT ON TABLE projects IS 'Projects (formerly organizations): own repositories; members hold a role on them';
COMMENT ON COLUMN projects.slug IS 'URL-safe identifier derived from the project name, e.g. "my-project"';
COMMENT ON TABLE project_members IS 'Project membership with roles (owner, maintainer, developer, viewer)';
COMMENT ON COLUMN project_members.updated_at IS 'Auto-updated by trigger on every UPDATE.';
COMMENT ON TABLE auth_groups IS 'Groups for organizing users and assigning permissions.';
COMMENT ON COLUMN auth_groups.project_id IS 'Project this group belongs to. NULL = system-level group (pre-multitenancy).';
COMMENT ON TABLE permission_targets IS 'Permission targets defining allowed actions on repositories. Each target belongs to exactly one project.';
COMMENT ON COLUMN permission_targets.project_id IS 'Project this permission target belongs to. NOT NULL.';
COMMENT ON COLUMN repositories.project_id IS 'Project this repository belongs to. NOT NULL; the system project means unassigned.';

-- Policy columns --------------------------------------------------------------

ALTER TABLE projects
    ADD COLUMN allowed_formats    text[],
    ADD COLUMN allowed_repo_types text[] NOT NULL DEFAULT '{hosted}',
    ADD COLUMN repo_name_prefix   text NOT NULL DEFAULT '',
    ADD COLUMN allow_public_repos boolean NOT NULL DEFAULT false,
    ADD COLUMN max_repositories   integer NOT NULL DEFAULT 50,
    ADD COLUMN token_max_ttl_days integer NOT NULL DEFAULT 365,
    ADD CONSTRAINT chk_project_repo_types CHECK (allowed_repo_types <@ ARRAY['hosted', 'proxy', 'group']),
    ADD CONSTRAINT chk_project_max_repositories CHECK (max_repositories >= 0),
    ADD CONSTRAINT chk_project_token_max_ttl CHECK (token_max_ttl_days >= 0);

COMMENT ON COLUMN projects.allowed_formats IS 'Formats self-service repositories may use; NULL = every format.';
COMMENT ON COLUMN projects.allowed_repo_types IS 'Repository types self-service repositories may use (hosted, proxy, group).';
COMMENT ON COLUMN projects.repo_name_prefix IS 'Self-service repository names must start with this prefix.';
COMMENT ON COLUMN projects.allow_public_repos IS 'Whether project repositories may be made anonymously readable (admin opt-in).';
COMMENT ON COLUMN projects.max_repositories IS 'Maximum number of repositories in the project; 0 = unlimited.';
COMMENT ON COLUMN projects.token_max_ttl_days IS 'Maximum lifetime of project tokens in days; 0 = no cap.';

UPDATE projects SET repo_name_prefix = slug || '-' WHERE id <> '00000000-0000-0000-0000-000000000001';

-- Member roles ----------------------------------------------------------------
-- owner stays owner, admin becomes maintainer, a plain member gets the role the
-- organization-wide member_role gave (read -> viewer, write -> developer,
-- admin -> maintainer). Organizations were administered by global admins only,
-- so no non-admin ever held these rights.

ALTER TABLE project_members DROP CONSTRAINT chk_org_member_role;

UPDATE project_members m SET role = CASE
        WHEN m.role = 'admin' THEN 'maintainer'
        WHEN m.role = 'member' THEN CASE p.member_role
            WHEN 'write' THEN 'developer'
            WHEN 'admin' THEN 'maintainer'
            ELSE 'viewer' END
        ELSE m.role END
    FROM projects p
    WHERE p.id = m.project_id AND m.role IN ('admin', 'member');

ALTER TABLE project_members
    ADD CONSTRAINT chk_project_member_role CHECK (role IN ('owner', 'maintainer', 'developer', 'viewer'));

ALTER TABLE projects DROP CONSTRAINT chk_org_member_role_level;
ALTER TABLE projects DROP COLUMN member_role;

-- Group grants ----------------------------------------------------------------

CREATE TABLE project_groups (
    project_id uuid NOT NULL,
    group_id uuid NOT NULL,
    role text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT project_groups_pkey PRIMARY KEY (project_id, group_id),
    CONSTRAINT chk_project_group_role CHECK (role IN ('viewer', 'developer', 'maintainer')),
    CONSTRAINT project_groups_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT project_groups_group_id_fkey FOREIGN KEY (group_id) REFERENCES auth_groups(id) ON DELETE CASCADE,
    CONSTRAINT project_groups_created_by_fkey FOREIGN KEY (created_by) REFERENCES auth_users(id) ON DELETE SET NULL
);

CREATE INDEX idx_project_groups_group ON project_groups (group_id);

COMMENT ON TABLE project_groups IS 'Groups attached to a project with a role (ownership stays personal).';

-- Audit actions ---------------------------------------------------------------
-- Added NOT VALID (existing rows are checked by VALIDATE CONSTRAINT in the
-- following 025_projects_validate.no_tx.sql). org.* stays legal: never emitted.

ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_action CHECK (action IN (
    -- Artifact lifecycle
    'artifact.uploaded', 'artifact.downloaded', 'artifact.deleted', 'artifact.moved',
    'artifact.promoted', 'artifact.quarantined', 'artifact.released',
    'artifact.restored', 'artifact.purged',
    -- Repository management
    'repository.created', 'repository.updated', 'repository.deleted',
    'repository.blocked', 'repository.unblocked',
    'repository.restored', 'repository.purged',
    -- Auth events
    'auth.login', 'auth.logout', 'auth.login_failed', 'auth.token_created',
    'auth.token_revoked', 'auth.password_changed', 'auth.sso_login',
    'auth.token_validate',
    -- Permission management
    'permission.target_created', 'permission.target_updated', 'permission.target_deleted',
    'permission.assigned', 'permission.revoked',
    -- Organization management (legacy, never emitted)
    'org.created', 'org.updated', 'org.deleted',
    'org.member_added', 'org.member_removed', 'org.member_role_changed',
    -- Project management
    'project.created', 'project.updated', 'project.deleted',
    'project.policy_changed', 'project.quota_changed',
    'project.member_added', 'project.member_removed', 'project.member_role_changed',
    'project.group_added', 'project.group_removed', 'project.group_role_changed',
    'project.token_created', 'project.token_revoked',
    'project.repository_created', 'project.repository_deleted', 'project.repository_transferred',
    'project.cleanup_policy_changed',
    -- System configuration
    'system.setting_changed', 'system.storage_backend_changed',
    -- Security scanning
    'scan.vuln_detected', 'scan.secret_detected', 'scan.quarantine_applied',
    -- Bundle/promotion
    'bundle.created', 'bundle.sealed', 'bundle.archived', 'bundle.promoted',
    -- HTTP audit (middleware logs method:path for generic request auditing)
    'http.request'
)) NOT VALID;
