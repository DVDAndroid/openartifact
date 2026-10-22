-- SPDX-FileCopyrightText: The OpenArtifact Authors
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Android application archive (format "android").
--
-- Numbered 900 so it never collides with upstream migrations; the migrator
-- applies any file not yet applied, whatever its position. Every table is new
-- (so indexes are built here, on empty tables); the only change to an existing
-- table is the format CHECK, added NOT VALID and validated in
-- 900_android_indexes.no_tx.sql.
--
-- Blobs are referenced by blobs.id with ON DELETE RESTRICT: the blob
-- collector cannot remove a file an Android row still points to.
--
-- Rows are deleted for good (no deleted_at): every table with that column must
-- be listed in trashSpecs, and trash support for android is a later change.

SET LOCAL lock_timeout = '10s';

ALTER TABLE repositories DROP CONSTRAINT IF EXISTS chk_repo_format;
ALTER TABLE repositories ADD CONSTRAINT chk_repo_format CHECK (format IN (
    'rpm', 'deb', 'maven', 'npm', 'docker', 'helm', 'pypi', 'nuget', 'go', 'cargo',
    'rubygems', 'raw', 'conda', 'conan', 'ivy', 'sbt', 'android')) NOT VALID;

-- One row per package name in a repository.
CREATE TABLE android_apps (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    package_name character varying(255) NOT NULL,
    label_override text,
    icon_override_blob_id uuid,
    notes text DEFAULT '' NOT NULL,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    signer_policy text DEFAULT 'warn' NOT NULL,
    upstream_kind text,
    upstream_ref text,
    upstream_latest text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT android_apps_pkey PRIMARY KEY (id),
    CONSTRAINT android_apps_repo_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE,
    CONSTRAINT android_apps_icon_blob_fkey FOREIGN KEY (icon_override_blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT chk_android_apps_policy CHECK (signer_policy IN ('allow', 'warn', 'reject')),
    CONSTRAINT chk_android_apps_notes CHECK (length(notes) <= 65536)
);

CREATE UNIQUE INDEX android_apps_key ON android_apps (repository_id, package_name);
CREATE INDEX idx_android_apps_tags ON android_apps USING gin (tags);

COMMENT ON TABLE android_apps IS 'Android apps (one per package name) of an android repository; notes and tags are hand-written.';

-- One row per stored file (APK or bundle).
CREATE TABLE android_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    app_id uuid NOT NULL,
    package_name character varying(255) NOT NULL,
    version_code bigint NOT NULL,
    version_name text DEFAULT '' NOT NULL,
    file_kind text DEFAULT 'apk' NOT NULL,
    blob_id uuid NOT NULL,
    sha256 character varying(64) NOT NULL,
    size_bytes bigint NOT NULL,
    filename text NOT NULL,

    -- Variant key: same package and versionCode can exist once per signer and ABI set.
    signer_sha256 character varying(64) DEFAULT '' NOT NULL,
    abis text[] DEFAULT '{}'::text[] NOT NULL,
    variant_key character varying(64) NOT NULL,

    -- Manifest and resources.
    label text DEFAULT '' NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    launcher_label text DEFAULT '' NOT NULL,
    min_sdk integer DEFAULT 1 NOT NULL,
    target_sdk integer DEFAULT 0 NOT NULL,
    compile_sdk integer DEFAULT 0 NOT NULL,
    max_sdk integer DEFAULT 0 NOT NULL,
    debuggable boolean DEFAULT false NOT NULL,
    manifest jsonb DEFAULT '{}'::jsonb NOT NULL,
    manifest_xml text DEFAULT '' NOT NULL,
    locales text[] DEFAULT '{}'::text[] NOT NULL,
    frameworks text[] DEFAULT '{}'::text[] NOT NULL,

    -- Signature.
    sig_schemes text[] DEFAULT '{}'::text[] NOT NULL,
    sig_verified boolean DEFAULT false NOT NULL,
    signature jsonb DEFAULT '{}'::jsonb NOT NULL,
    content_digest character varying(64) DEFAULT '' NOT NULL,

    -- Native code, dependencies, packaging.
    native_libs jsonb DEFAULT '[]'::jsonb NOT NULL,
    page_size_16k boolean DEFAULT true NOT NULL,
    dependencies jsonb DEFAULT '[]'::jsonb NOT NULL,
    build_info jsonb DEFAULT '{}'::jsonb NOT NULL,
    zip_checks jsonb DEFAULT '{}'::jsonb NOT NULL,
    entries jsonb DEFAULT '[]'::jsonb NOT NULL,
    warnings jsonb DEFAULT '[]'::jsonb NOT NULL,

    -- Provenance.
    origin text DEFAULT 'unknown' NOT NULL,
    parent_id uuid,
    patch_info jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_url text DEFAULT '' NOT NULL,
    original_sha256 character varying(64) DEFAULT '' NOT NULL,

    -- Hand-written.
    changelog text DEFAULT '' NOT NULL,
    notes text DEFAULT '' NOT NULL,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    channel text DEFAULT '' NOT NULL,
    pinned boolean DEFAULT false NOT NULL,

    download_count bigint DEFAULT 0 NOT NULL,
    created_by text DEFAULT '' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT android_versions_pkey PRIMARY KEY (id),
    CONSTRAINT android_versions_repo_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE,
    CONSTRAINT android_versions_app_fkey FOREIGN KEY (app_id) REFERENCES android_apps(id) ON DELETE CASCADE,
    CONSTRAINT android_versions_blob_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT android_versions_parent_fkey FOREIGN KEY (parent_id) REFERENCES android_versions(id) ON DELETE SET NULL,
    CONSTRAINT chk_android_versions_origin CHECK (origin IN ('built', 'patched', 'original', 'unknown')),
    CONSTRAINT chk_android_versions_kind CHECK (file_kind IN ('apk', 'apks', 'xapk', 'apkm')),
    CONSTRAINT chk_android_versions_notes CHECK (length(notes) <= 65536 AND length(changelog) <= 65536)
);

CREATE UNIQUE INDEX android_versions_variant_key
    ON android_versions (repository_id, package_name, version_code, variant_key);
CREATE INDEX idx_android_versions_app ON android_versions (app_id, version_code DESC, created_at DESC);
CREATE INDEX idx_android_versions_repo_created ON android_versions (repository_id, created_at DESC, id);
CREATE INDEX idx_android_versions_blob ON android_versions (blob_id);
CREATE INDEX idx_android_versions_sha ON android_versions (repository_id, sha256);
CREATE INDEX idx_android_versions_signer ON android_versions (repository_id, signer_sha256);
CREATE INDEX idx_android_versions_parent ON android_versions (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_android_versions_tags ON android_versions USING gin (tags);

COMMENT ON TABLE android_versions IS 'Stored Android files (APK or bundle) with the metadata pkg/apk extracted at upload.';
COMMENT ON COLUMN android_versions.variant_key IS 'sha256 of signer fingerprint and sorted ABIs: a patched APK keeps the original versionCode, so the signer is part of the key.';
COMMENT ON COLUMN android_versions.content_digest IS 'Digest of entry names, CRCs and sizes without signature files: equal values are the same build signed with different keys.';

-- Files attached to a version (mapping, patch bundle, original APK, ...).
CREATE TABLE android_files (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    version_id uuid NOT NULL,
    kind text NOT NULL,
    name text NOT NULL,
    blob_id uuid NOT NULL,
    sha256 character varying(64) NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT android_files_pkey PRIMARY KEY (id),
    CONSTRAINT android_files_version_fkey FOREIGN KEY (version_id) REFERENCES android_versions(id) ON DELETE CASCADE,
    CONSTRAINT android_files_blob_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT,
    CONSTRAINT chk_android_files_kind CHECK (kind IN ('mapping', 'symbols', 'patch_bundle', 'patch_options', 'source', 'screenshot', 'original_apk', 'other'))
);

CREATE INDEX idx_android_files_version ON android_files (version_id);
CREATE INDEX idx_android_files_blob ON android_files (blob_id);

-- Icons extracted from a version (one row per role and density).
CREATE TABLE android_icons (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    version_id uuid NOT NULL,
    role text NOT NULL,
    density text DEFAULT '' NOT NULL,
    width integer DEFAULT 0 NOT NULL,
    height integer DEFAULT 0 NOT NULL,
    mime text NOT NULL,
    source_path text DEFAULT '' NOT NULL,
    resource_name text DEFAULT '' NOT NULL,
    color text DEFAULT '' NOT NULL,
    adaptive boolean DEFAULT false NOT NULL,
    renderable boolean DEFAULT false NOT NULL,
    blob_id uuid,
    sha256 character varying(64) DEFAULT '' NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT android_icons_pkey PRIMARY KEY (id),
    CONSTRAINT android_icons_version_fkey FOREIGN KEY (version_id) REFERENCES android_versions(id) ON DELETE CASCADE,
    CONSTRAINT android_icons_blob_fkey FOREIGN KEY (blob_id) REFERENCES blobs(id) ON DELETE RESTRICT
);

CREATE INDEX idx_android_icons_version ON android_icons (version_id);
CREATE INDEX idx_android_icons_blob ON android_icons (blob_id) WHERE blob_id IS NOT NULL;

-- Known signing keys (public certificates or fingerprints only, never private keys).
CREATE TABLE android_signers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    sha256 character varying(64) NOT NULL,
    label text DEFAULT '' NOT NULL,
    trust text NOT NULL,
    app_id uuid,
    notes text DEFAULT '' NOT NULL,
    certificate jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT android_signers_pkey PRIMARY KEY (id),
    CONSTRAINT android_signers_repo_fkey FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE,
    CONSTRAINT android_signers_app_fkey FOREIGN KEY (app_id) REFERENCES android_apps(id) ON DELETE CASCADE,
    CONSTRAINT chk_android_signers_trust CHECK (trust IN ('mine', 'original', 'third_party', 'untrusted')),
    CONSTRAINT chk_android_signers_sha CHECK (sha256 ~ '^[0-9a-f]{64}$')
);

-- A key is known once per repository and app (NULL app: repository-wide).
CREATE UNIQUE INDEX android_signers_key ON android_signers (repository_id, sha256, coalesce(app_id, '00000000-0000-0000-0000-000000000000'::uuid));
