-- SPDX-FileCopyrightText: The OpenArtifact Authors
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- OpenArtifact database schema.
--
-- Single foundational migration: creates every table, constraint, index and
-- trigger, plus the seed rows the application relies on. Future schema changes
-- are added as new numbered files next to this one.
--
-- Files ending in .no_tx.sql are executed outside a transaction (needed for
-- CREATE INDEX CONCURRENTLY and similar statements).

-- Shared trigger function: keeps updated_at current on every UPDATE.
CREATE FUNCTION update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ============================================================================
-- Identity, organizations and access control
-- ============================================================================

CREATE TABLE auth_users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    username character varying(64) NOT NULL,
    email character varying(254),
    password_hash text,
    full_name text,
    disabled boolean DEFAULT false NOT NULL,
    must_change_password boolean DEFAULT false NOT NULL,
    last_login_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    external_id text,
    provider_slug text,
    sso_managed_groups boolean DEFAULT false NOT NULL,
    CONSTRAINT auth_users_pkey PRIMARY KEY (id),
    CONSTRAINT auth_users_email_key UNIQUE (email),
    CONSTRAINT auth_users_username_key UNIQUE (username)
);

CREATE UNIQUE INDEX auth_users_sso_unique ON auth_users (external_id, provider_slug) WHERE ((external_id IS NOT NULL) AND (provider_slug IS NOT NULL));

CREATE TRIGGER update_auth_users_updated_at
    BEFORE UPDATE ON auth_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE auth_users IS 'User accounts for authentication and authorization';

CREATE TABLE organizations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    display_name character varying(255),
    description text,
    owner_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    slug character varying(64) NOT NULL,
    CONSTRAINT organizations_pkey PRIMARY KEY (id),
    CONSTRAINT organizations_name_key UNIQUE (name),
    CONSTRAINT organizations_slug_unique UNIQUE (slug),
    CONSTRAINT organizations_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES auth_users(id) ON DELETE SET NULL
);

CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE organizations IS 'Organizations for multi-tenancy isolation';

COMMENT ON COLUMN organizations.slug IS 'URL-safe identifier for the organization, e.g. "my-org"';

CREATE TABLE organization_members (
    org_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role character varying(50) DEFAULT 'member' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT organization_members_pkey PRIMARY KEY (org_id, user_id),
    CONSTRAINT chk_org_member_role CHECK (role IN ('owner', 'admin', 'member')),
    CONSTRAINT organization_members_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT organization_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_org_members_org_id ON organization_members (org_id);

CREATE INDEX idx_org_members_user_id ON organization_members (user_id);

CREATE TRIGGER update_organization_members_updated_at
    BEFORE UPDATE ON organization_members
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE organization_members IS 'Organization membership with roles (owner, admin, member)';

COMMENT ON COLUMN organization_members.updated_at IS 'Auto-updated by trigger on every UPDATE.';

CREATE TABLE auth_groups (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(64) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid,
    CONSTRAINT auth_groups_pkey PRIMARY KEY (id),
    CONSTRAINT auth_groups_org_name_unique UNIQUE (org_id, name),
    CONSTRAINT auth_groups_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_groups_org_id ON auth_groups (org_id);

CREATE TRIGGER update_auth_groups_updated_at
    BEFORE UPDATE ON auth_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE auth_groups IS 'Org-scoped groups for organizing users and assigning permissions.';

COMMENT ON COLUMN auth_groups.org_id IS 'Organization this group belongs to. NULL = system-level group (pre-multitenancy).';

CREATE TABLE auth_user_groups (
    user_id uuid NOT NULL,
    group_id uuid NOT NULL,
    is_sso_managed boolean DEFAULT false NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT auth_user_groups_pkey PRIMARY KEY (user_id, group_id),
    CONSTRAINT auth_user_groups_group_id_fkey FOREIGN KEY (group_id) REFERENCES auth_groups(id) ON DELETE CASCADE,
    CONSTRAINT auth_user_groups_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
);

CREATE TRIGGER update_auth_user_groups_updated_at
    BEFORE UPDATE ON auth_user_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE auth_user_groups IS 'Many-to-many membership between users and groups';

COMMENT ON COLUMN auth_user_groups.updated_at IS 'Auto-updated by trigger on every UPDATE.';

CREATE TABLE auth_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    ip_address inet,
    user_agent text,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT auth_sessions_pkey PRIMARY KEY (id),
    CONSTRAINT auth_sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT auth_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_sessions_token_hash ON auth_sessions (token_hash);

CREATE INDEX idx_auth_sessions_user_active ON auth_sessions (user_id, expires_at);

CREATE INDEX idx_auth_sessions_user_id ON auth_sessions (user_id);

COMMENT ON TABLE auth_sessions IS 'Browser session tokens stored as SHA-256 hashes';

CREATE TABLE auth_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    name character varying(128) NOT NULL,
    kind character varying(16) NOT NULL,
    token_hash text NOT NULL,
    scopes jsonb DEFAULT '[]' NOT NULL,
    expires_at timestamp with time zone,
    last_used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    CONSTRAINT auth_tokens_pkey PRIMARY KEY (id),
    CONSTRAINT auth_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT auth_tokens_kind_check CHECK (kind IN ('access', 'deploy')),
    CONSTRAINT auth_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens (token_hash);

CREATE INDEX idx_auth_tokens_user_id ON auth_tokens (user_id);

COMMENT ON TABLE auth_tokens IS 'Personal access tokens and deploy tokens stored as SHA-256 hashes';

CREATE TABLE auth_sso_providers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    slug character varying(64) NOT NULL,
    kind character varying(8) NOT NULL,
    display_name text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    oidc_config jsonb,
    saml_config jsonb,
    claim_mapping jsonb,
    group_mapping jsonb DEFAULT '{}' NOT NULL,
    auto_provision_users boolean DEFAULT true NOT NULL,
    link_by_email boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    permission_mapping jsonb DEFAULT '{}' NOT NULL,
    CONSTRAINT auth_sso_providers_pkey PRIMARY KEY (id),
    CONSTRAINT auth_sso_providers_slug_key UNIQUE (slug),
    CONSTRAINT auth_sso_providers_kind_check CHECK (kind IN ('oidc', 'saml')),
    CONSTRAINT chk_sso_config_matches_kind CHECK ((((kind = 'oidc') AND (oidc_config IS NOT NULL)) OR ((kind = 'saml') AND (saml_config IS NOT NULL))))
);

CREATE TRIGGER update_auth_sso_providers_updated_at
    BEFORE UPDATE ON auth_sso_providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE auth_sso_providers IS 'SSO provider configuration for OIDC and SAML identity providers';

CREATE TABLE permission_targets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(128) NOT NULL,
    repository_filter text NOT NULL,
    path_pattern text NOT NULL,
    actions text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid NOT NULL,
    CONSTRAINT permission_targets_pkey PRIMARY KEY (id),
    CONSTRAINT permission_targets_org_name_unique UNIQUE (org_id, name),
    CONSTRAINT permission_targets_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE INDEX idx_permission_targets_org_id ON permission_targets (org_id);

CREATE TRIGGER update_permission_targets_updated_at
    BEFORE UPDATE ON permission_targets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE permission_targets IS 'Org-scoped permission targets defining allowed actions on repositories. Each target belongs to exactly one organization.';

COMMENT ON COLUMN permission_targets.org_id IS 'Organization this permission target belongs to. NOT NULL.';

CREATE TABLE permission_assignments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    permission_target_id uuid NOT NULL,
    principal_type character varying(8) NOT NULL,
    principal_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT permission_assignments_pkey PRIMARY KEY (id),
    CONSTRAINT permission_assignments_principal_type_check CHECK (principal_type IN ('user', 'group')),
    CONSTRAINT permission_assignments_permission_target_id_fkey FOREIGN KEY (permission_target_id) REFERENCES permission_targets(id) ON DELETE CASCADE
);

CREATE INDEX idx_permission_assignments_principal ON permission_assignments (principal_id, principal_type);

CREATE UNIQUE INDEX idx_permission_assignments_unique ON permission_assignments (permission_target_id, principal_type, principal_id);

COMMENT ON TABLE permission_assignments IS 'Assignments of permission targets to users or groups';

-- ============================================================================
-- Storage backends and content-addressed blobs
-- ============================================================================

CREATE TABLE storage_backends (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(128) NOT NULL,
    kind character varying(32) NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    config jsonb DEFAULT '{}' NOT NULL,
    status character varying(20) DEFAULT 'unknown' NOT NULL,
    status_message text,
    last_checked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT storage_backends_pkey PRIMARY KEY (id),
    CONSTRAINT storage_backends_name_key UNIQUE (name),
    CONSTRAINT chk_storage_backend_kind CHECK (kind IN ('filesystem', 's3', 'garage', 'seaweedfs', 'oci', 'gcs', 'azure')),
    CONSTRAINT chk_storage_backend_status CHECK (status IN ('unknown', 'healthy', 'degraded', 'unreachable'))
);

CREATE UNIQUE INDEX idx_storage_backends_default ON storage_backends (is_default) WHERE (is_default = true);

CREATE TRIGGER update_storage_backends_updated_at
    BEFORE UPDATE ON storage_backends
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE storage_backends IS 'Configured storage backends for artifact persistence';

CREATE TABLE blobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    sha256 character varying(64) NOT NULL,
    size_bytes bigint NOT NULL,
    media_type text DEFAULT 'application/octet-stream' NOT NULL,
    storage_backend_id uuid,
    blob_key text NOT NULL,
    ref_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT blobs_pkey PRIMARY KEY (id),
    CONSTRAINT blobs_sha256_key UNIQUE (sha256),
    CONSTRAINT blobs_ref_count_check CHECK ((ref_count >= 0)),
    CONSTRAINT blobs_storage_backend_id_fkey FOREIGN KEY (storage_backend_id) REFERENCES storage_backends(id) ON DELETE SET NULL
);

CREATE INDEX idx_blobs_ref_count_zero ON blobs (id) WHERE (ref_count = 0);

CREATE INDEX idx_blobs_sha256 ON blobs (sha256);

CREATE INDEX idx_blobs_storage_backend ON blobs (storage_backend_id) WHERE (storage_backend_id IS NOT NULL);

COMMENT ON TABLE blobs IS 'Global content-addressed blob store. One row per unique sha256. Cross-format, cross-repo deduplication. GC via ref_count. All format tables reference blobs via blob_id FK.';

COMMENT ON COLUMN blobs.blob_key IS 'Key used to retrieve the blob from the configured storage backend.';

COMMENT ON COLUMN blobs.ref_count IS 'Number of format table rows referencing this blob. When 0, the blob is a GC candidate.';

-- ============================================================================
-- Repositories
-- ============================================================================

CREATE TABLE repositories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    format character varying(50) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    allow_full_unpublish boolean DEFAULT false NOT NULL,
    public boolean DEFAULT false NOT NULL,
    repo_type character varying(20) DEFAULT 'hosted' NOT NULL,
    upstream_url text,
    upstream_username text,
    upstream_password_enc text,
    cache_ttl_seconds integer DEFAULT 3600 NOT NULL,
    negative_cache_ttl_seconds integer DEFAULT 60 NOT NULL,
    blocked boolean DEFAULT false NOT NULL,
    org_id uuid NOT NULL,
    CONSTRAINT repositories_pkey PRIMARY KEY (id),
    CONSTRAINT repositories_org_name_unique UNIQUE (org_id, name),
    CONSTRAINT chk_repo_format CHECK (format IN ('rpm', 'deb', 'maven', 'npm', 'docker', 'helm', 'pypi', 'nuget', 'go', 'cargo', 'rubygems', 'raw', 'conda', 'conan', 'ivy', 'sbt')),
    CONSTRAINT chk_repo_type CHECK (repo_type IN ('hosted', 'proxy', 'group')),
    CONSTRAINT fk_repositories_org FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE RESTRICT
);

CREATE INDEX idx_repositories_format ON repositories (format);

CREATE INDEX idx_repositories_name ON repositories (name);

CREATE INDEX idx_repositories_org_id ON repositories (org_id);

CREATE TRIGGER update_repositories_updated_at
    BEFORE UPDATE ON repositories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE repositories IS 'Parent table for all repository instances (deb, rpm, maven, npm, etc.)';

COMMENT ON COLUMN repositories.format IS 'Package format: rpm, deb, maven, npm, docker, helm, pypi, nuget, go, cargo, rubygems, raw, conda, conan, ivy, sbt. Constrained by chk_repo_format.';

COMMENT ON COLUMN repositories.allow_full_unpublish IS 'Per-repo override of the 72-hour npm unpublish grace period';

COMMENT ON COLUMN repositories.public IS 'Publicly readable without authentication';

COMMENT ON COLUMN repositories.repo_type IS 'Repository type: hosted (stores artifacts), proxy (caches upstream), group (aggregates multiple repos). Constrained by chk_repo_type.';

COMMENT ON COLUMN repositories.upstream_username IS 'Proxy upstream username. Stored in plaintext (not sensitive). Pair with upstream_password_enc for authentication.';

COMMENT ON COLUMN repositories.upstream_password_enc IS 'AES-256-GCM encrypted proxy upstream password. Plaintext MUST NOT be stored here. Encrypt before write, decrypt after read.';

COMMENT ON COLUMN repositories.org_id IS 'Organization this repository belongs to. NOT NULL — all repos must be org-scoped.';

CREATE TABLE repository_group_members (
    group_repo_id uuid NOT NULL,
    member_repo_id uuid NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT repository_group_members_pkey PRIMARY KEY (group_repo_id, member_repo_id),
    CONSTRAINT repository_group_members_group_repo_id_fkey FOREIGN KEY (group_repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
    CONSTRAINT repository_group_members_member_repo_id_fkey FOREIGN KEY (member_repo_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_repo_group_members_group ON repository_group_members (group_repo_id);

CREATE TABLE proxy_cache_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    cache_key text NOT NULL,
    blob_key text,
    content_type text,
    status_code integer DEFAULT 200 NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    hit_count bigint DEFAULT 0 NOT NULL,
    last_hit_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT proxy_cache_entries_pkey PRIMARY KEY (id),
    CONSTRAINT proxy_cache_entries_repo_id_cache_key_key UNIQUE (repository_id, cache_key),
    CONSTRAINT proxy_cache_entries_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_proxy_cache_expires ON proxy_cache_entries (expires_at);

CREATE INDEX idx_proxy_cache_repository_key ON proxy_cache_entries (repository_id, cache_key);

CREATE TRIGGER update_proxy_cache_entries_updated_at
    BEFORE UPDATE ON proxy_cache_entries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE proxy_cache_entries IS 'Cache entries for proxy repositories';

COMMENT ON COLUMN proxy_cache_entries.hit_count IS 'Cache hit counter; BIGINT to avoid overflow on high-traffic repos';

CREATE TABLE cleanup_policies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    keep_last_n integer,
    keep_since_days integer,
    keep_downloaded_since_days integer,
    name_pattern text,
    schedule_cron text DEFAULT '0 2 * * *' NOT NULL,
    last_run_at timestamp with time zone,
    last_run_deleted integer,
    last_run_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cleanup_policies_pkey PRIMARY KEY (id),
    CONSTRAINT cleanup_policies_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_cleanup_policies_enabled ON cleanup_policies (enabled) WHERE (enabled = true);

CREATE INDEX idx_cleanup_policies_repository_id ON cleanup_policies (repository_id);

CREATE TRIGGER update_cleanup_policies_updated_at
    BEFORE UPDATE ON cleanup_policies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE cleanup_policies IS 'Cleanup/retention policies for automatic artifact deletion';

CREATE TABLE storage_quotas (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    max_size_bytes bigint DEFAULT 0 NOT NULL,
    max_artifact_count bigint DEFAULT 0 NOT NULL,
    warn_threshold_percent integer DEFAULT 80 NOT NULL,
    current_size_bytes bigint DEFAULT 0 NOT NULL,
    current_artifact_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT storage_quotas_pkey PRIMARY KEY (id),
    CONSTRAINT storage_quotas_repo_id_key UNIQUE (repository_id),
    CONSTRAINT storage_quotas_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_storage_quotas_repository_id ON storage_quotas (repository_id);

CREATE TRIGGER update_storage_quotas_updated_at
    BEFORE UPDATE ON storage_quotas
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE storage_quotas IS 'Storage quotas and usage tracking per repository';

-- ============================================================================
-- Format: Maven / Ivy / sbt
-- ============================================================================

CREATE TABLE maven_artifacts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    group_id text NOT NULL,
    artifact_id_coord text NOT NULL,
    version text NOT NULL,
    classifier text DEFAULT '' NOT NULL,
    extension text NOT NULL,
    snapshot_timestamp text,
    snapshot_build_number integer DEFAULT 0 NOT NULL,
    is_snapshot boolean DEFAULT false NOT NULL,
    repo_path text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english'::regconfig, ((((COALESCE(group_id, ''::text) || ' '::text) || COALESCE(artifact_id_coord, ''::text)) || ' '::text) || COALESCE(version, ''::text)))) STORED,
    blob_id uuid NOT NULL,
    CONSTRAINT maven_artifacts_pkey PRIMARY KEY (id),
    CONSTRAINT maven_artifacts_repository_id_repo_path_key UNIQUE (repository_id, repo_path),
    CONSTRAINT maven_artifacts_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT maven_artifacts_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_maven_artifacts_ga ON maven_artifacts (repository_id, group_id, artifact_id_coord);

CREATE INDEX idx_maven_artifacts_gav ON maven_artifacts (repository_id, group_id, artifact_id_coord, version);

CREATE INDEX idx_maven_artifacts_repository_id ON maven_artifacts (repository_id);

CREATE INDEX maven_artifacts_search_idx ON maven_artifacts USING gin (search_tsv);

CREATE TRIGGER update_maven_artifacts_updated_at
    BEFORE UPDATE ON maven_artifacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN maven_artifacts.snapshot_timestamp IS 'Snapshot timestamp string (e.g. "20240101.120000"). NULL for release artifacts.';

COMMENT ON COLUMN maven_artifacts.snapshot_build_number IS 'Snapshot build number. 0 = not a snapshot or build number not tracked. NULL would be cleaner but 0 is kept for backward compatibility.';

CREATE TABLE ivy_artifacts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    organisation character varying(255) NOT NULL,
    module character varying(255) NOT NULL,
    revision character varying(100) NOT NULL,
    artifact_name character varying(255) NOT NULL,
    artifact_type character varying(50) DEFAULT 'jar' NOT NULL,
    artifact_ext character varying(50) DEFAULT 'jar' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT ivy_artifacts_pkey PRIMARY KEY (id),
    CONSTRAINT ivy_artifacts_repo_id_organisation_module_revision_artifact_key UNIQUE (repository_id, organisation, module, revision, artifact_name, artifact_ext),
    CONSTRAINT ivy_artifacts_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT ivy_artifacts_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_ivy_artifacts_repo_org_module ON ivy_artifacts (repository_id, organisation, module);

CREATE INDEX idx_ivy_artifacts_repository_id ON ivy_artifacts (repository_id);

CREATE TRIGGER update_ivy_artifacts_updated_at
    BEFORE UPDATE ON ivy_artifacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE sbt_artifacts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    group_id character varying(255) NOT NULL,
    artifact_id character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    scala_version character varying(20) DEFAULT '' NOT NULL,
    filename character varying(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT sbt_artifacts_pkey PRIMARY KEY (id),
    CONSTRAINT sbt_artifacts_repo_id_group_id_artifact_id_version_filename_key UNIQUE (repository_id, group_id, artifact_id, version, filename),
    CONSTRAINT sbt_artifacts_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT sbt_artifacts_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_sbt_artifacts_repo_group_artifact ON sbt_artifacts (repository_id, group_id, artifact_id);

CREATE INDEX idx_sbt_artifacts_repository_id ON sbt_artifacts (repository_id);

CREATE TRIGGER update_sbt_artifacts_updated_at
    BEFORE UPDATE ON sbt_artifacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Format: npm
-- ============================================================================

CREATE TABLE npm_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    scope text DEFAULT '' NOT NULL,
    name text NOT NULL,
    full_name text NOT NULL,
    latest_version text DEFAULT '' NOT NULL,
    packument_stale boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT npm_packages_pkey PRIMARY KEY (id),
    CONSTRAINT npm_packages_repository_id_full_name_key UNIQUE (repository_id, full_name),
    CONSTRAINT npm_packages_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_npm_packages_repository_id ON npm_packages (repository_id);

CREATE TRIGGER update_npm_packages_updated_at
    BEFORE UPDATE ON npm_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE npm_packages IS 'One row per npm package (scoped or unscoped) within a repository';

COMMENT ON COLUMN npm_packages.scope IS 'Scope prefix without @, empty string for unscoped packages';

COMMENT ON COLUMN npm_packages.full_name IS 'Full package name including scope, e.g. @scope/name or name';

COMMENT ON COLUMN npm_packages.packument_stale IS 'True when latest_version denormalization needs refresh';

CREATE TABLE npm_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    package_id uuid NOT NULL,
    version text NOT NULL,
    manifest_json jsonb NOT NULL,
    integrity text NOT NULL,
    shasum text NOT NULL,
    tarball_filename text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english'::regconfig, ((((COALESCE((manifest_json ->> 'name'::text), ''::text) || ' '::text) || COALESCE(version, ''::text)) || ' '::text) || COALESCE((manifest_json ->> 'description'::text), ''::text)))) STORED,
    blob_id uuid NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT npm_versions_pkey PRIMARY KEY (id),
    CONSTRAINT npm_versions_package_id_version_key UNIQUE (package_id, version),
    CONSTRAINT npm_versions_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT npm_versions_package_id_fkey FOREIGN KEY (package_id) REFERENCES npm_packages(id) ON DELETE CASCADE
);

CREATE INDEX idx_npm_versions_package_id ON npm_versions (package_id);

CREATE INDEX npm_versions_search_idx ON npm_versions USING gin (search_tsv);

CREATE TRIGGER update_npm_versions_updated_at
    BEFORE UPDATE ON npm_versions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE npm_versions IS 'One row per published version of an npm package';

COMMENT ON COLUMN npm_versions.manifest_json IS 'Full package.json manifest as stored in the packument';

COMMENT ON COLUMN npm_versions.integrity IS 'Subresource Integrity string, e.g. sha512-<base64>';

COMMENT ON COLUMN npm_versions.updated_at IS 'Auto-updated by trigger on every UPDATE.';

CREATE TABLE npm_dist_tags (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    package_id uuid NOT NULL,
    tag text NOT NULL,
    version text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT npm_dist_tags_pkey PRIMARY KEY (id),
    CONSTRAINT npm_dist_tags_package_id_tag_key UNIQUE (package_id, tag),
    CONSTRAINT npm_dist_tags_package_id_fkey FOREIGN KEY (package_id) REFERENCES npm_packages(id) ON DELETE CASCADE
);

CREATE INDEX idx_npm_dist_tags_package_id ON npm_dist_tags (package_id);

CREATE TRIGGER update_npm_dist_tags_updated_at
    BEFORE UPDATE ON npm_dist_tags
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE npm_dist_tags IS 'Distribution tags for npm packages, e.g. latest, next, beta';

COMMENT ON COLUMN npm_dist_tags.tag IS 'Tag name, e.g. latest';

COMMENT ON COLUMN npm_dist_tags.version IS 'Version string the tag currently points to';

-- ============================================================================
-- Format: PyPI, NuGet, Go modules, Cargo, RubyGems, Conda, Conan, Helm, raw
-- ============================================================================

CREATE TABLE pypi_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    filename character varying(500) NOT NULL,
    package_type character varying(20) NOT NULL,
    python_version character varying(50),
    requires_python character varying(100),
    summary text,
    author character varying(255),
    author_email character varying(255),
    license text,
    keywords text,
    classifiers text[],
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT pypi_packages_pkey PRIMARY KEY (id),
    CONSTRAINT pypi_packages_repo_id_name_version_filename_key UNIQUE (repository_id, name, version, filename),
    CONSTRAINT pypi_packages_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT pypi_packages_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_pypi_packages_repo_name ON pypi_packages (repository_id, name);

CREATE INDEX idx_pypi_packages_repository_id ON pypi_packages (repository_id);

CREATE TRIGGER update_pypi_packages_updated_at
    BEFORE UPDATE ON pypi_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE pypi_packages IS 'PyPI Python packages';

CREATE TABLE nuget_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    package_id character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    title character varying(255),
    description text,
    summary text,
    authors text,
    owners text,
    tags text,
    project_url text,
    license_url text,
    icon_url text,
    require_license_acceptance boolean DEFAULT false NOT NULL,
    is_prerelease boolean DEFAULT false NOT NULL,
    min_client_version character varying(50),
    download_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT nuget_packages_pkey PRIMARY KEY (id),
    CONSTRAINT nuget_packages_repo_id_package_id_version_key UNIQUE (repository_id, package_id, version),
    CONSTRAINT nuget_packages_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT nuget_packages_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_nuget_packages_repo_id ON nuget_packages (repository_id, lower((package_id)::text));

CREATE INDEX idx_nuget_packages_repository_id ON nuget_packages (repository_id);

CREATE TRIGGER update_nuget_packages_updated_at
    BEFORE UPDATE ON nuget_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE nuget_packages IS 'NuGet .NET packages';

COMMENT ON COLUMN nuget_packages.download_count IS 'Total downloads; BIGINT to avoid overflow on popular packages';

CREATE TABLE gomod_modules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    module_path text NOT NULL,
    version character varying(100) NOT NULL,
    go_mod_content text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    zip_blob_id uuid,
    mod_blob_id uuid,
    CONSTRAINT gomod_modules_pkey PRIMARY KEY (id),
    CONSTRAINT gomod_modules_repo_id_module_path_version_key UNIQUE (repository_id, module_path, version),
    CONSTRAINT gomod_modules_mod_blob_id_fkey FOREIGN KEY (mod_blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT gomod_modules_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE,
    CONSTRAINT gomod_modules_zip_blob_id_fkey FOREIGN KEY (zip_blob_id) REFERENCES blobs(id) ON DELETE RESTRICT
);

CREATE INDEX idx_gomod_modules_repo_path ON gomod_modules (repository_id, module_path);

CREATE INDEX idx_gomod_modules_repository_id ON gomod_modules (repository_id);

CREATE TRIGGER update_gomod_modules_updated_at
    BEFORE UPDATE ON gomod_modules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE gomod_modules IS 'Go module proxy packages';

CREATE TABLE cargo_crates (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    description text,
    homepage text,
    documentation text,
    repository text,
    license text,
    keywords text[],
    categories text[],
    authors text[],
    readme text,
    metadata_json jsonb DEFAULT '{}' NOT NULL,
    yanked boolean DEFAULT false NOT NULL,
    download_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT cargo_crates_pkey PRIMARY KEY (id),
    CONSTRAINT cargo_crates_repo_id_name_version_key UNIQUE (repository_id, name, version),
    CONSTRAINT cargo_crates_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT cargo_crates_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_cargo_crates_repo_name ON cargo_crates (repository_id, name);

CREATE INDEX idx_cargo_crates_repository_id ON cargo_crates (repository_id);

CREATE TRIGGER update_cargo_crates_updated_at
    BEFORE UPDATE ON cargo_crates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE cargo_crates IS 'Rust/Cargo packages.';

COMMENT ON COLUMN cargo_crates.download_count IS 'Total downloads; BIGINT to avoid overflow on popular crates';

CREATE TABLE rubygems_gems (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    platform character varying(50) DEFAULT 'ruby' NOT NULL,
    description text,
    summary text,
    homepage text,
    authors text[],
    licenses text[],
    dependencies jsonb DEFAULT '[]' NOT NULL,
    metadata_json jsonb DEFAULT '{}' NOT NULL,
    yanked boolean DEFAULT false NOT NULL,
    download_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT rubygems_gems_pkey PRIMARY KEY (id),
    CONSTRAINT rubygems_gems_repo_id_name_version_platform_key UNIQUE (repository_id, name, version, platform),
    CONSTRAINT rubygems_gems_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT rubygems_gems_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_rubygems_gems_repo_name ON rubygems_gems (repository_id, name);

CREATE INDEX idx_rubygems_gems_repository_id ON rubygems_gems (repository_id);

CREATE TRIGGER update_rubygems_gems_updated_at
    BEFORE UPDATE ON rubygems_gems
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN rubygems_gems.download_count IS 'Total downloads; BIGINT to avoid overflow on popular gems';

CREATE TABLE conda_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    build_string character varying(255) DEFAULT '' NOT NULL,
    platform character varying(50) DEFAULT 'noarch' NOT NULL,
    depends text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT conda_packages_pkey PRIMARY KEY (id),
    CONSTRAINT conda_packages_repo_id_name_version_build_string_platform_key UNIQUE (repository_id, name, version, build_string, platform),
    CONSTRAINT conda_packages_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT conda_packages_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_conda_packages_repo_name ON conda_packages (repository_id, name);

CREATE INDEX idx_conda_packages_repository_id ON conda_packages (repository_id);

CREATE TRIGGER update_conda_packages_updated_at
    BEFORE UPDATE ON conda_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE conan_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    user_channel character varying(255) DEFAULT '_/_' NOT NULL,
    recipe_revision character varying(64) DEFAULT '0' NOT NULL,
    conanfile text,
    conaninfo text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid,
    CONSTRAINT conan_packages_pkey PRIMARY KEY (id),
    CONSTRAINT conan_packages_repo_id_name_version_user_channel_recipe_rev_key UNIQUE (repository_id, name, version, user_channel, recipe_revision),
    CONSTRAINT conan_packages_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT conan_packages_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_conan_packages_repo_name ON conan_packages (repository_id, name);

CREATE INDEX idx_conan_packages_repository_id ON conan_packages (repository_id);

CREATE TRIGGER update_conan_packages_updated_at
    BEFORE UPDATE ON conan_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE helm_charts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(100) NOT NULL,
    app_version character varying(100),
    description text,
    keywords text[],
    home text,
    sources text[],
    maintainers jsonb,
    icon text,
    api_version character varying(20) DEFAULT 'v2' NOT NULL,
    type character varying(20) DEFAULT 'application' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT helm_charts_pkey PRIMARY KEY (id),
    CONSTRAINT helm_charts_repo_id_name_version_key UNIQUE (repository_id, name, version),
    CONSTRAINT helm_charts_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT helm_charts_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_helm_charts_repo_name ON helm_charts (repository_id, name);

CREATE INDEX idx_helm_charts_repository_id ON helm_charts (repository_id);

CREATE TRIGGER update_helm_charts_updated_at
    BEFORE UPDATE ON helm_charts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE helm_charts IS 'Helm chart packages';

CREATE TABLE raw_artifacts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    path text NOT NULL,
    content_type character varying(255) DEFAULT 'application/octet-stream' NOT NULL,
    metadata jsonb DEFAULT '{}' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT raw_artifacts_pkey PRIMARY KEY (id),
    CONSTRAINT raw_artifacts_repo_id_path_key UNIQUE (repository_id, path),
    CONSTRAINT raw_artifacts_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT raw_artifacts_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_raw_artifacts_repo_path ON raw_artifacts (repository_id, path);

CREATE INDEX idx_raw_artifacts_repository_id ON raw_artifacts (repository_id);

CREATE TRIGGER update_raw_artifacts_updated_at
    BEFORE UPDATE ON raw_artifacts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Format: Docker / OCI
-- ============================================================================

CREATE TABLE docker_blobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    digest text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT docker_blobs_pkey PRIMARY KEY (id),
    CONSTRAINT docker_blobs_repository_id_digest_key UNIQUE (repository_id, digest),
    CONSTRAINT docker_blobs_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT docker_blobs_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_docker_blobs_digest ON docker_blobs (digest);

CREATE INDEX idx_docker_blobs_repository_id ON docker_blobs (repository_id);

COMMENT ON TABLE docker_blobs IS 'OCI/Docker blob registry: per-repo existence index. Content (size, media_type, storage key) is in the global blobs table via blob_id. Shared blobs are deduplicated across repositories.';

COMMENT ON COLUMN docker_blobs.digest IS 'OCI digest in "sha256:<hex>" format. Used by OCI registry protocol.';

CREATE TABLE docker_manifests (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    digest text NOT NULL,
    media_type text NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    subject_digest text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    blob_id uuid NOT NULL,
    CONSTRAINT docker_manifests_pkey PRIMARY KEY (id),
    CONSTRAINT docker_manifests_repository_id_digest_key UNIQUE (repository_id, digest),
    CONSTRAINT docker_manifests_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT docker_manifests_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_docker_manifests_digest ON docker_manifests (digest);

CREATE INDEX idx_docker_manifests_repository_id ON docker_manifests (repository_id);

CREATE TRIGGER update_docker_manifests_updated_at
    BEFORE UPDATE ON docker_manifests
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE docker_manifests IS 'OCI/Docker image manifests indexed by digest';

CREATE TABLE docker_tags (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name text NOT NULL,
    manifest_digest text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT docker_tags_pkey PRIMARY KEY (id),
    CONSTRAINT docker_tags_repository_id_name_key UNIQUE (repository_id, name),
    CONSTRAINT docker_tags_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_docker_tags_manifest_digest ON docker_tags (repository_id, manifest_digest);

CREATE INDEX idx_docker_tags_repository_id ON docker_tags (repository_id);

CREATE TRIGGER update_docker_tags_updated_at
    BEFORE UPDATE ON docker_tags
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE docker_tags IS 'Named tags pointing to manifests in a repository';

CREATE TABLE docker_uploads (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    upload_uuid text NOT NULL,
    "offset" bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT docker_uploads_pkey PRIMARY KEY (id),
    CONSTRAINT docker_uploads_upload_uuid_key UNIQUE (upload_uuid),
    CONSTRAINT docker_uploads_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_docker_uploads_repository_id ON docker_uploads (repository_id);

CREATE TRIGGER update_docker_uploads_updated_at
    BEFORE UPDATE ON docker_uploads
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE docker_uploads IS 'In-progress chunked blob upload sessions';

-- ============================================================================
-- Format: RPM
-- ============================================================================

CREATE TABLE rpm_repos (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    base_arch text NOT NULL,
    metadata_stale boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    signing_key_id text,
    signing_key_fingerprint text,
    CONSTRAINT rpm_repos_pkey PRIMARY KEY (id),
    CONSTRAINT rpm_repos_repository_id_base_arch_key UNIQUE (repository_id, base_arch),
    CONSTRAINT fk_rpm_repos_repository FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_rpm_repos_repository_id ON rpm_repos (repository_id);

CREATE INDEX idx_rpm_repos_signing_key ON rpm_repos (signing_key_id) WHERE (signing_key_id IS NOT NULL);

CREATE INDEX idx_rpm_repos_stale ON rpm_repos (metadata_stale) WHERE (metadata_stale = true);

CREATE TRIGGER update_rpm_repos_updated_at
    BEFORE UPDATE ON rpm_repos
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE repository_metadata (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    path text NOT NULL,
    blob_sha256 text NOT NULL,
    blob_size bigint NOT NULL,
    last_generated timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT repo_metadata_pkey PRIMARY KEY (id),
    CONSTRAINT repo_metadata_repository_id_path_key UNIQUE (repository_id, path),
    CONSTRAINT fk_repo_metadata_repository FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_repository_metadata_path ON repository_metadata (repository_id, path);

CREATE INDEX idx_repository_metadata_repository_id ON repository_metadata (repository_id);

CREATE TABLE rpm_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    blob_id uuid NOT NULL,
    name text NOT NULL,
    version text NOT NULL,
    release text DEFAULT '' NOT NULL,
    epoch integer DEFAULT 0 NOT NULL,
    arch text NOT NULL,
    summary text,
    description text,
    license text,
    url text,
    vendor text,
    group_name text,
    packager text,
    source_rpm text,
    build_host text,
    build_time timestamp with time zone,
    filename text,
    package_size bigint,
    installed_size bigint,
    archive_size bigint,
    search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english'::regconfig, ((((((COALESCE(name, ''::text) || ' '::text) || COALESCE(description, ''::text)) || ' '::text) || COALESCE(summary, ''::text)) || ' '::text) || COALESCE(license, ''::text)))) STORED,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT rpm_packages_pkey PRIMARY KEY (id),
    CONSTRAINT rpm_packages_repository_id_name_version_release_epoch_arch_key UNIQUE (repository_id, name, version, release, epoch, arch),
    CONSTRAINT rpm_packages_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT rpm_packages_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_rpm_packages_name ON rpm_packages (repository_id, name);

CREATE INDEX idx_rpm_packages_repository_id ON rpm_packages (repository_id);

CREATE INDEX idx_rpm_packages_search ON rpm_packages USING gin (search_tsv);

CREATE TRIGGER update_rpm_packages_updated_at
    BEFORE UPDATE ON rpm_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE rpm_packages IS 'RPM packages. Canonical table for RPM format. blob_id FK to global blobs table.';

-- ============================================================================
-- Format: Debian
-- ============================================================================

CREATE TABLE debian_distributions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    codename character varying(255),
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT debian_distributions_pkey PRIMARY KEY (id),
    CONSTRAINT debian_distributions_repository_id_name_key UNIQUE (repository_id, name),
    CONSTRAINT debian_distributions_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_debian_distributions_repo ON debian_distributions (repository_id);

CREATE TRIGGER update_debian_distributions_updated_at
    BEFORE UPDATE ON debian_distributions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE debian_components (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    distribution_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT debian_components_pkey PRIMARY KEY (id),
    CONSTRAINT debian_components_distribution_id_name_key UNIQUE (distribution_id, name),
    CONSTRAINT debian_components_distribution_id_fkey FOREIGN KEY (distribution_id) REFERENCES debian_distributions(id) ON DELETE CASCADE
);

CREATE INDEX idx_debian_components_dist ON debian_components (distribution_id);

CREATE TRIGGER update_debian_components_updated_at
    BEFORE UPDATE ON debian_components
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE debian_architectures (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    distribution_id uuid NOT NULL,
    name character varying(50) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT debian_architectures_pkey PRIMARY KEY (id),
    CONSTRAINT debian_architectures_distribution_id_name_key UNIQUE (distribution_id, name),
    CONSTRAINT debian_architectures_distribution_id_fkey FOREIGN KEY (distribution_id) REFERENCES debian_distributions(id) ON DELETE CASCADE
);

CREATE INDEX idx_debian_architectures_dist ON debian_architectures (distribution_id);

CREATE TRIGGER update_debian_architectures_updated_at
    BEFORE UPDATE ON debian_architectures
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE deb_packages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    distribution_id uuid NOT NULL,
    component_id uuid NOT NULL,
    blob_id uuid NOT NULL,
    name text NOT NULL,
    version text NOT NULL,
    arch text NOT NULL,
    summary text,
    description text,
    license text,
    maintainer text,
    homepage text,
    filename text,
    installed_size bigint,
    search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english'::regconfig, ((((((COALESCE(name, ''::text) || ' '::text) || COALESCE(description, ''::text)) || ' '::text) || COALESCE(summary, ''::text)) || ' '::text) || COALESCE(license, ''::text)))) STORED,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT deb_packages_pkey PRIMARY KEY (id),
    CONSTRAINT deb_packages_repository_id_name_version_arch_component_id_key UNIQUE (repository_id, name, version, arch, component_id),
    CONSTRAINT deb_packages_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT deb_packages_component_id_fkey FOREIGN KEY (component_id) REFERENCES debian_components(id) ON DELETE CASCADE,
    CONSTRAINT deb_packages_distribution_id_fkey FOREIGN KEY (distribution_id) REFERENCES debian_distributions(id) ON DELETE CASCADE,
    CONSTRAINT deb_packages_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_deb_packages_distribution ON deb_packages (distribution_id);

CREATE INDEX idx_deb_packages_name ON deb_packages (repository_id, name);

CREATE INDEX idx_deb_packages_repository_id ON deb_packages (repository_id);

CREATE INDEX idx_deb_packages_search ON deb_packages USING gin (search_tsv);

CREATE TRIGGER update_deb_packages_updated_at
    BEFORE UPDATE ON deb_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE deb_packages IS 'Debian packages. Canonical table for Debian format. Scoped to distribution+component for proper Debian model. blob_id FK to global blobs table.';

-- ============================================================================
-- Artifact references, properties and scanning (vulnerability, license, secrets, SBOM, AI)
-- ============================================================================

CREATE TABLE artifact_refs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    artifact_path text NOT NULL,
    format character varying(50) NOT NULL,
    sha256 character varying(64),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT artifact_refs_pkey PRIMARY KEY (id),
    CONSTRAINT artifact_refs_repository_id_artifact_path_key UNIQUE (repository_id, artifact_path),
    CONSTRAINT artifact_refs_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_artifact_refs_repository_id ON artifact_refs (repository_id);

CREATE INDEX idx_artifact_refs_sha256 ON artifact_refs (sha256) WHERE (sha256 IS NOT NULL);

COMMENT ON TABLE artifact_refs IS 'Universal scan target anchors. One row per (repository, artifact_path). artifact_path = HTTP request path used to fetch the artifact. Format-agnostic. Created on scan submission. Cascades on repository deletion.';

CREATE TABLE artifact_properties (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    key character varying(255) NOT NULL,
    value text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    artifact_ref_id uuid,
    CONSTRAINT artifact_properties_pkey PRIMARY KEY (id),
    CONSTRAINT artifact_properties_artifact_ref_id_fkey FOREIGN KEY (artifact_ref_id) REFERENCES artifact_refs(id) ON DELETE CASCADE,
    CONSTRAINT artifact_properties_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_artifact_properties_ref_id ON artifact_properties (artifact_ref_id);

CREATE INDEX idx_artifact_properties_repo_key ON artifact_properties (repository_id, key);

CREATE TRIGGER update_artifact_properties_updated_at
    BEFORE UPDATE ON artifact_properties
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE artifact_properties IS 'Custom key-value metadata for artifacts';

COMMENT ON COLUMN artifact_properties.artifact_ref_id IS 'FK to artifact_refs.';

CREATE TABLE scan_jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    artifact_ref_id uuid NOT NULL,
    scan_type character varying(20) NOT NULL,
    scanner character varying(50),
    status character varying(20) DEFAULT 'pending' NOT NULL,
    error_message text,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT scan_jobs_pkey PRIMARY KEY (id),
    CONSTRAINT scan_jobs_artifact_ref_id_scan_type_scanner_key UNIQUE (artifact_ref_id, scan_type, scanner),
    CONSTRAINT scan_jobs_scan_type_check CHECK (scan_type IN ('vuln', 'license', 'secret', 'sbom', 'ai')),
    CONSTRAINT scan_jobs_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    CONSTRAINT scan_jobs_artifact_ref_id_fkey FOREIGN KEY (artifact_ref_id) REFERENCES artifact_refs(id) ON DELETE CASCADE
);

CREATE INDEX idx_scan_jobs_artifact_ref ON scan_jobs (artifact_ref_id);

CREATE INDEX idx_scan_jobs_pending ON scan_jobs (scan_type, created_at) WHERE status IN ('pending', 'running');

CREATE INDEX idx_scan_jobs_status ON scan_jobs (status) WHERE status IN ('pending', 'running');

CREATE TRIGGER update_scan_jobs_updated_at
    BEFORE UPDATE ON scan_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE scan_jobs IS 'Parent table for all scan work (vuln, license, secret, sbom, ai). Result payloads are in: vuln_results, license_results, secret_results, sbom_results, ai_results.';

COMMENT ON COLUMN scan_jobs.scanner IS 'Scanner implementation: osv, trivy, grype, internal, etc. NULL = default for type.';

CREATE TABLE vuln_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scan_job_id uuid NOT NULL,
    vulnerabilities jsonb DEFAULT '[]' NOT NULL,
    vuln_count integer DEFAULT 0 NOT NULL,
    critical_count integer DEFAULT 0 NOT NULL,
    high_count integer DEFAULT 0 NOT NULL,
    medium_count integer DEFAULT 0 NOT NULL,
    low_count integer DEFAULT 0 NOT NULL,
    quarantined boolean DEFAULT false NOT NULL,
    package_name text,
    package_version text,
    ecosystem text,
    CONSTRAINT vuln_results_pkey PRIMARY KEY (id),
    CONSTRAINT vuln_results_scan_job_id_key UNIQUE (scan_job_id),
    CONSTRAINT vuln_results_scan_job_id_fkey FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_vuln_results_critical ON vuln_results (critical_count) WHERE (critical_count > 0);

CREATE INDEX idx_vuln_results_quarantined ON vuln_results (scan_job_id) WHERE (quarantined = true);

CREATE INDEX idx_vuln_results_scan_job ON vuln_results (scan_job_id);

COMMENT ON TABLE vuln_results IS 'Vulnerability scan payloads. FK to scan_jobs.';

CREATE TABLE license_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scan_job_id uuid NOT NULL,
    licenses text[] DEFAULT '{}'::text[] NOT NULL,
    spdx_expressions text[] DEFAULT '{}'::text[] NOT NULL,
    CONSTRAINT license_results_pkey PRIMARY KEY (id),
    CONSTRAINT license_results_scan_job_id_key UNIQUE (scan_job_id),
    CONSTRAINT license_results_scan_job_id_fkey FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_license_results_scan_job ON license_results (scan_job_id);

COMMENT ON TABLE license_results IS 'License scan payloads. FK to scan_jobs.';

CREATE TABLE secret_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scan_job_id uuid NOT NULL,
    findings jsonb DEFAULT '[]' NOT NULL,
    finding_count integer DEFAULT 0 NOT NULL,
    quarantined boolean DEFAULT false NOT NULL,
    CONSTRAINT secret_results_pkey PRIMARY KEY (id),
    CONSTRAINT secret_results_scan_job_id_key UNIQUE (scan_job_id),
    CONSTRAINT secret_results_scan_job_id_fkey FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_secret_results_quarantined ON secret_results (scan_job_id) WHERE (quarantined = true);

CREATE INDEX idx_secret_results_scan_job ON secret_results (scan_job_id);

COMMENT ON TABLE secret_results IS 'Secret detection payloads. FK to scan_jobs.';

CREATE TABLE sbom_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scan_job_id uuid NOT NULL,
    sbom_format character varying(20) NOT NULL,
    spec_version character varying(10) NOT NULL,
    blob_id uuid,
    component_count integer DEFAULT 0 NOT NULL,
    CONSTRAINT sbom_results_pkey PRIMARY KEY (id),
    CONSTRAINT sbom_results_scan_job_id_sbom_format_spec_version_key UNIQUE (scan_job_id, sbom_format, spec_version),
    CONSTRAINT sbom_results_blob_id_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE SET NULL,
    CONSTRAINT sbom_results_scan_job_id_fkey FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_sbom_results_blob ON sbom_results (blob_id) WHERE (blob_id IS NOT NULL);

CREATE INDEX idx_sbom_results_scan_job ON sbom_results (scan_job_id);

COMMENT ON TABLE sbom_results IS 'SBOM report payloads. FK to scan_jobs. Content in blobs (not inline).';

CREATE TABLE ai_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scan_job_id uuid NOT NULL,
    summary text DEFAULT '' NOT NULL,
    tags jsonb DEFAULT '[]' NOT NULL,
    license_status character varying(20) DEFAULT 'UNKNOWN' NOT NULL,
    license_name text DEFAULT '' NOT NULL,
    security_notes text DEFAULT '' NOT NULL,
    quarantine_recommended boolean DEFAULT false NOT NULL,
    model_used text DEFAULT '' NOT NULL,
    prompt_tokens integer DEFAULT 0 NOT NULL,
    completion_tokens integer DEFAULT 0 NOT NULL,
    CONSTRAINT ai_results_pkey PRIMARY KEY (id),
    CONSTRAINT ai_results_scan_job_id_key UNIQUE (scan_job_id),
    CONSTRAINT ai_results_license_status_check CHECK (license_status IN ('SAFE', 'REVIEW_REQUIRED', 'UNKNOWN')),
    CONSTRAINT ai_results_scan_job_id_fkey FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_ai_results_license_status ON ai_results (license_status);

CREATE INDEX idx_ai_results_quarantine ON ai_results (scan_job_id) WHERE (quarantine_recommended = true);

CREATE INDEX idx_ai_results_scan_job ON ai_results (scan_job_id);

CREATE INDEX idx_ai_results_tags ON ai_results USING gin (tags);

COMMENT ON TABLE ai_results IS 'AI analysis payloads. FK to scan_jobs.';

CREATE TABLE license_policies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid,
    name character varying(255) NOT NULL,
    description text,
    action character varying(20) DEFAULT 'warn' NOT NULL,
    allowed_licenses text[] DEFAULT '{}'::text[] NOT NULL,
    denied_licenses text[] DEFAULT '{}'::text[] NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT license_policies_pkey PRIMARY KEY (id),
    CONSTRAINT chk_license_policy_action CHECK (action IN ('warn', 'block', 'allow')),
    CONSTRAINT license_policies_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_license_policies_repository_id ON license_policies (repository_id);

CREATE TRIGGER update_license_policies_updated_at
    BEFORE UPDATE ON license_policies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE license_policies IS 'License compliance policies (repo-specific and global)';

-- ============================================================================
-- Release bundles, promotion and package deltas
-- ============================================================================

CREATE TABLE promotion_pipelines (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    stages jsonb DEFAULT '[]' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    creator_id uuid,
    CONSTRAINT promotion_pipelines_pkey PRIMARY KEY (id),
    CONSTRAINT promotion_pipelines_name_key UNIQUE (name),
    CONSTRAINT promotion_pipelines_creator_id_fkey FOREIGN KEY (creator_id) REFERENCES auth_users(id) ON DELETE SET NULL
);

CREATE TRIGGER update_promotion_pipelines_updated_at
    BEFORE UPDATE ON promotion_pipelines
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE promotion_pipelines IS 'Named, ordered promotion stage sequences reusable across bundles';

COMMENT ON COLUMN promotion_pipelines.creator_id IS 'FK to auth_users.';

CREATE TABLE release_bundles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    version character varying(255) NOT NULL,
    description text,
    state character varying(20) DEFAULT 'draft' NOT NULL,
    pipeline_id uuid,
    current_stage character varying(255),
    source_query jsonb,
    sealed_at timestamp with time zone,
    archived_at timestamp with time zone,
    gpg_key_id text,
    gpg_signature text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    creator_id uuid,
    sealer_id uuid,
    archiver_id uuid,
    CONSTRAINT release_bundles_pkey PRIMARY KEY (id),
    CONSTRAINT release_bundles_name_version_key UNIQUE (name, version),
    CONSTRAINT release_bundles_state_check CHECK (state IN ('draft', 'sealed', 'archived')),
    CONSTRAINT release_bundles_archiver_id_fkey FOREIGN KEY (archiver_id) REFERENCES auth_users(id) ON DELETE SET NULL,
    CONSTRAINT release_bundles_creator_id_fkey FOREIGN KEY (creator_id) REFERENCES auth_users(id) ON DELETE SET NULL,
    CONSTRAINT release_bundles_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES promotion_pipelines(id) ON DELETE SET NULL,
    CONSTRAINT release_bundles_sealer_id_fkey FOREIGN KEY (sealer_id) REFERENCES auth_users(id) ON DELETE SET NULL
);

CREATE INDEX idx_release_bundles_creator ON release_bundles (creator_id) WHERE (creator_id IS NOT NULL);

CREATE INDEX idx_release_bundles_name ON release_bundles (name);

CREATE INDEX idx_release_bundles_pipeline ON release_bundles (pipeline_id) WHERE (pipeline_id IS NOT NULL);

CREATE INDEX idx_release_bundles_state ON release_bundles (state);

CREATE TRIGGER update_release_bundles_updated_at
    BEFORE UPDATE ON release_bundles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE release_bundles IS 'Versioned, named collections of artifacts moving through a promotion pipeline';

COMMENT ON COLUMN release_bundles.creator_id IS 'FK to auth_users.';

COMMENT ON COLUMN release_bundles.sealer_id IS 'FK to auth_users.';

COMMENT ON COLUMN release_bundles.archiver_id IS 'FK to auth_users.';

CREATE TABLE release_bundle_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    bundle_id uuid NOT NULL,
    source_repo_id uuid NOT NULL,
    sha256 text NOT NULL,
    artifact_path text NOT NULL,
    format character varying(50) NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    added_at timestamp with time zone DEFAULT now() NOT NULL,
    artifact_ref_id uuid,
    adder_id uuid,
    CONSTRAINT release_bundle_items_pkey PRIMARY KEY (id),
    CONSTRAINT release_bundle_items_bundle_id_sha256_key UNIQUE (bundle_id, sha256),
    CONSTRAINT release_bundle_items_adder_id_fkey FOREIGN KEY (adder_id) REFERENCES auth_users(id) ON DELETE SET NULL,
    CONSTRAINT release_bundle_items_artifact_ref_id_fkey FOREIGN KEY (artifact_ref_id) REFERENCES artifact_refs(id) ON DELETE SET NULL,
    CONSTRAINT release_bundle_items_bundle_id_fkey FOREIGN KEY (bundle_id) REFERENCES release_bundles(id) ON DELETE CASCADE,
    CONSTRAINT release_bundle_items_source_repo_id_fkey FOREIGN KEY (source_repo_id) REFERENCES repositories(id) ON DELETE RESTRICT
);

CREATE INDEX idx_bundle_items_bundle_id ON release_bundle_items (bundle_id);

CREATE INDEX idx_bundle_items_sha256 ON release_bundle_items (sha256);

CREATE INDEX idx_bundle_items_source_repo_id ON release_bundle_items (source_repo_id);

COMMENT ON TABLE release_bundle_items IS 'Artifact references (by SHA-256) that constitute a release bundle';

COMMENT ON COLUMN release_bundle_items.adder_id IS 'FK to auth_users.';

CREATE TABLE bundle_promotions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    bundle_id uuid NOT NULL,
    from_stage character varying(255),
    to_stage character varying(255) NOT NULL,
    target_repo_id uuid NOT NULL,
    status character varying(20) DEFAULT 'in_progress' NOT NULL,
    notes text,
    error_message text,
    item_count integer DEFAULT 0 NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    promoter_id uuid,
    CONSTRAINT bundle_promotions_pkey PRIMARY KEY (id),
    CONSTRAINT bundle_promotions_status_check CHECK (status IN ('in_progress', 'success', 'failed')),
    CONSTRAINT bundle_promotions_bundle_id_fkey FOREIGN KEY (bundle_id) REFERENCES release_bundles(id) ON DELETE CASCADE,
    CONSTRAINT bundle_promotions_promoter_id_fkey FOREIGN KEY (promoter_id) REFERENCES auth_users(id) ON DELETE SET NULL,
    CONSTRAINT bundle_promotions_target_repo_id_fkey FOREIGN KEY (target_repo_id) REFERENCES repositories(id) ON DELETE RESTRICT
);

CREATE INDEX idx_bundle_promotions_bundle ON bundle_promotions (bundle_id);

CREATE INDEX idx_bundle_promotions_promoter ON bundle_promotions (promoter_id) WHERE (promoter_id IS NOT NULL);

CREATE INDEX idx_bundle_promotions_status ON bundle_promotions (status) WHERE (status = 'in_progress');

COMMENT ON TABLE bundle_promotions IS 'Immutable audit trail of every bundle promotion attempt';

COMMENT ON COLUMN bundle_promotions.promoter_id IS 'FK to auth_users.';

CREATE TABLE package_deltas (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    from_sha256 character varying(64) NOT NULL,
    from_filename character varying(512) NOT NULL,
    to_sha256 character varying(64) NOT NULL,
    to_filename character varying(512) NOT NULL,
    delta_sha256 character varying(64) NOT NULL,
    delta_size bigint DEFAULT 0 NOT NULL,
    format character varying(16) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT package_deltas_pkey PRIMARY KEY (id),
    CONSTRAINT package_deltas_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_package_deltas_to_sha256 ON package_deltas (repository_id, to_sha256);

CREATE UNIQUE INDEX idx_package_deltas_from_to ON package_deltas (repository_id, from_sha256, to_sha256);

COMMENT ON TABLE package_deltas IS 'Pre-computed bsdiff patches between package versions for bandwidth-efficient updates';

-- ============================================================================
-- Operations: audit log, webhooks, backups, system settings
-- ============================================================================

CREATE TABLE audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    action character varying(50) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    org_id uuid,
    repository_id uuid,
    actor_id uuid,
    actor_type character varying(20),
    details_json jsonb DEFAULT '{}' NOT NULL,
    resource_type character varying(50),
    resource_id text,
    ip_address inet,
    CONSTRAINT audit_logs_pkey PRIMARY KEY (id),
    CONSTRAINT audit_logs_actor_type_check CHECK (actor_type IN ('user', 'system', 'token', 'sso')),
    CONSTRAINT chk_audit_action CHECK (action IN (
        -- Artifact lifecycle
        'artifact.uploaded', 'artifact.downloaded', 'artifact.deleted', 'artifact.moved',
        'artifact.promoted', 'artifact.quarantined', 'artifact.released',
        -- Repository management
        'repository.created', 'repository.updated', 'repository.deleted',
        'repository.blocked', 'repository.unblocked',
        -- Auth events
        'auth.login', 'auth.logout', 'auth.login_failed', 'auth.token_created',
        'auth.token_revoked', 'auth.password_changed', 'auth.sso_login',
        'auth.token_validate',
        -- Permission management
        'permission.target_created', 'permission.target_updated', 'permission.target_deleted',
        'permission.assigned', 'permission.revoked',
        -- Organization management
        'org.created', 'org.updated', 'org.deleted',
        'org.member_added', 'org.member_removed', 'org.member_role_changed',
        -- System configuration
        'system.setting_changed', 'system.storage_backend_changed',
        -- Security scanning
        'scan.vuln_detected', 'scan.secret_detected', 'scan.quarantine_applied',
        -- Bundle/promotion
        'bundle.created', 'bundle.sealed', 'bundle.archived', 'bundle.promoted',
        -- HTTP audit (middleware logs method:path for generic request auditing)
        'http.request'
    )),
    CONSTRAINT audit_logs_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES auth_users(id) ON DELETE SET NULL,
    CONSTRAINT audit_logs_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE SET NULL,
    CONSTRAINT audit_logs_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE SET NULL
);

CREATE INDEX idx_audit_logs_action ON audit_logs (action);

CREATE INDEX idx_audit_logs_actor_id ON audit_logs (actor_id) WHERE (actor_id IS NOT NULL);

CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);

CREATE INDEX idx_audit_logs_org_id ON audit_logs (org_id) WHERE (org_id IS NOT NULL);

CREATE INDEX idx_audit_logs_repository_id ON audit_logs (repository_id) WHERE (repository_id IS NOT NULL);

CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id) WHERE (resource_type IS NOT NULL);

COMMENT ON TABLE audit_logs IS 'Compliance-grade audit trail. actor_id FK to auth_users, details_json JSONB.';

COMMENT ON COLUMN audit_logs.actor_id IS 'FK to auth_users.';

COMMENT ON COLUMN audit_logs.details_json IS 'Structured event details as JSONB.';

CREATE TABLE webhooks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid,
    name character varying(255) NOT NULL,
    url text NOT NULL,
    secret_enc text,
    enabled boolean DEFAULT true NOT NULL,
    events text[] DEFAULT '{}'::text[] NOT NULL,
    content_type character varying(50) DEFAULT 'application/json' NOT NULL,
    ssl_verify boolean DEFAULT true NOT NULL,
    max_retries integer DEFAULT 3 NOT NULL,
    timeout_seconds integer DEFAULT 30 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT webhooks_pkey PRIMARY KEY (id),
    CONSTRAINT webhooks_repo_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_webhooks_enabled ON webhooks (enabled) WHERE (enabled = true);

CREATE INDEX idx_webhooks_repository_id ON webhooks (repository_id);

CREATE TRIGGER update_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE webhooks IS 'Webhook subscriptions for artifact lifecycle events';

COMMENT ON COLUMN webhooks.secret_enc IS 'AES-256-GCM encrypted HMAC-SHA256 signing secret. Plaintext MUST NOT be stored here. Encrypt before write, decrypt after read.';

CREATE TABLE webhook_deliveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    webhook_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status character varying(20) DEFAULT 'pending' NOT NULL,
    response_status integer,
    response_body text,
    attempt_count integer DEFAULT 0 NOT NULL,
    next_retry_at timestamp with time zone,
    delivered_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT webhook_deliveries_pkey PRIMARY KEY (id),
    CONSTRAINT chk_webhook_delivery_status CHECK (status IN ('pending', 'success', 'failed')),
    CONSTRAINT webhook_deliveries_webhook_id_fkey FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);

CREATE INDEX idx_webhook_deliveries_retry ON webhook_deliveries (next_retry_at) WHERE (status = 'pending');

CREATE INDEX idx_webhook_deliveries_retry_worker ON webhook_deliveries (webhook_id, next_retry_at) WHERE status IN ('pending', 'failed');

CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries (status) WHERE status IN ('pending', 'failed');

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id);

CREATE TRIGGER update_webhook_deliveries_updated_at
    BEFORE UPDATE ON webhook_deliveries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE webhook_deliveries IS 'Webhook delivery attempts and history';

CREATE TABLE backup_jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    status character varying(20) DEFAULT 'pending' NOT NULL,
    type character varying(20) DEFAULT 'full' NOT NULL,
    storage_path text,
    size_bytes bigint,
    error_message text,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT backup_jobs_pkey PRIMARY KEY (id),
    CONSTRAINT chk_backup_status CHECK (status IN ('pending', 'running', 'success', 'failed')),
    CONSTRAINT chk_backup_type CHECK (type IN ('full', 'incremental'))
);

CREATE INDEX idx_backup_jobs_status ON backup_jobs (status);

CREATE TRIGGER update_backup_jobs_updated_at
    BEFORE UPDATE ON backup_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE backup_jobs IS 'Backup job tracking';

CREATE TABLE system_settings (
    key character varying(255) NOT NULL,
    value text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT system_settings_pkey PRIMARY KEY (key)
);

CREATE TRIGGER update_system_settings_updated_at
    BEFORE UPDATE ON system_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Seed data
-- ============================================================================

-- Default organization; repositories without an explicit org belong to it.
INSERT INTO organizations (id, name, display_name, description, slug)
VALUES ('00000000-0000-0000-0000-000000000001', 'system', 'System',
        'Default organization for repositories created before multi-tenancy was introduced', 'system');

-- Built-in permission targets (bound to the bootstrap admin / anonymous users).
INSERT INTO permission_targets (name, repository_filter, path_pattern, actions, org_id)
VALUES
    ('admin-all', '.*', '.*', '{read,write,delete,manage,annotate}', '00000000-0000-0000-0000-000000000001'),
    ('anonymous-read', '.*', '.*', '{read}', '00000000-0000-0000-0000-000000000001');

-- Default local filesystem storage backend.
INSERT INTO storage_backends (name, kind, is_default, config, status)
VALUES ('local', 'filesystem', true, '{"path": "./storage"}', 'unknown');
