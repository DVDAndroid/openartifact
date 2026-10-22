// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"time"
)

// Android signer policies (android_apps.signer_policy).
const (
	AndroidPolicyAllow  = "allow"
	AndroidPolicyWarn   = "warn"
	AndroidPolicyReject = "reject"
)

// Android version origins (android_versions.origin).
const (
	AndroidOriginBuilt    = "built"
	AndroidOriginPatched  = "patched"
	AndroidOriginOriginal = "original"
	AndroidOriginUnknown  = "unknown"
)

// Android signer trust levels (android_signers.trust).
const (
	AndroidTrustMine       = "mine"
	AndroidTrustOriginal   = "original"
	AndroidTrustThirdParty = "third_party"
	AndroidTrustUntrusted  = "untrusted"
)

// AndroidApp is one package name in an android repository.
type AndroidApp struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	PackageName    string    `json:"package_name"`
	LabelOverride  string    `json:"label_override,omitempty"`
	Notes          string    `json:"notes"`
	Tags           []string  `json:"tags"`
	SignerPolicy   string    `json:"signer_policy"`
	UpstreamKind   string    `json:"upstream_kind,omitempty"`
	UpstreamRef    string    `json:"upstream_ref,omitempty"`
	UpstreamLatest string    `json:"upstream_latest,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AndroidAppSummary is an app with the aggregates the app list shows.
type AndroidAppSummary struct {
	AndroidApp

	Label        string    `json:"label"`
	VersionCount int       `json:"version_count"`
	Origins      []string  `json:"origins"`
	Signers      []string  `json:"signers"`
	TotalSize    int64     `json:"total_size_bytes"`
	LatestID     string    `json:"latest_version_id"`
	LatestName   string    `json:"latest_version_name"`
	LatestCode   int64     `json:"latest_version_code"`
	LatestSigner string    `json:"latest_signer_sha256"`
	LatestOrigin string    `json:"latest_origin"`
	LatestAt     time.Time `json:"latest_created_at"`
}

// AndroidAppFilter narrows ListAndroidApps.
type AndroidAppFilter struct {
	Query   string
	Tags    []string // AND
	Origin  string
	Signer  string
	Sort    string // name | updated | versions | size
	Limit   int
	Offset  int
	Reverse bool
}

// AndroidVersion is one stored APK or bundle. The jsonb columns are kept as
// raw JSON: the API passes them through and pkg/apk owns their shape. The
// heavy ones (ManifestXML, Entries, Manifest, Signature) are only loaded by
// GetAndroidVersion.
type AndroidVersion struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	AppID        string `json:"app_id"`
	PackageName  string `json:"package_name"`
	VersionCode  int64  `json:"version_code"`
	VersionName  string `json:"version_name"`
	FileKind     string `json:"file_kind"`
	BlobID       string `json:"-"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"size_bytes"`
	Filename     string `json:"filename"`

	SignerSHA256 string   `json:"signer_sha256"`
	ABIs         []string `json:"abis"`
	VariantKey   string   `json:"variant_key"`

	Label         string          `json:"label"`
	Labels        json.RawMessage `json:"labels,omitempty"`
	LauncherLabel string          `json:"launcher_label,omitempty"`
	MinSDK        int             `json:"min_sdk"`
	TargetSDK     int             `json:"target_sdk"`
	CompileSDK    int             `json:"compile_sdk,omitempty"`
	MaxSDK        int             `json:"max_sdk,omitempty"`
	Debuggable    bool            `json:"debuggable"`
	Locales       []string        `json:"locales"`
	Frameworks    []string        `json:"frameworks"`

	SigSchemes    []string `json:"sig_schemes"`
	SigVerified   bool     `json:"sig_verified"`
	ContentDigest string   `json:"content_digest"`
	PageSize16K   bool     `json:"page_size_16k"`

	Origin         string          `json:"origin"`
	ParentID       string          `json:"parent_id,omitempty"`
	PatchInfo      json.RawMessage `json:"patch_info,omitempty"`
	SourceURL      string          `json:"source_url,omitempty"`
	OriginalSHA256 string          `json:"original_sha256,omitempty"`

	Changelog     string    `json:"changelog"`
	Notes         string    `json:"notes"`
	Tags          []string  `json:"tags"`
	Channel       string    `json:"channel"`
	Pinned        bool      `json:"pinned"`
	DownloadCount int64     `json:"download_count"`
	CreatedBy     string    `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Detail-only columns.
	Manifest     json.RawMessage `json:"manifest,omitempty"`
	ManifestXML  string          `json:"-"`
	Signature    json.RawMessage `json:"signature,omitempty"`
	NativeLibs   json.RawMessage `json:"native_libs,omitempty"`
	Dependencies json.RawMessage `json:"dependencies,omitempty"`
	BuildInfo    json.RawMessage `json:"build_info,omitempty"`
	ZipChecks    json.RawMessage `json:"zip_checks,omitempty"`
	Entries      json.RawMessage `json:"entries,omitempty"`
	Warnings     json.RawMessage `json:"warnings,omitempty"`
}

// AndroidVersionFilter narrows ListAndroidVersions.
type AndroidVersionFilter struct {
	AppID  string
	Origin string
	Signer string
	Tags   []string // AND
	Limit  int
	Offset int
}

// AndroidVersionMeta is the hand-written part of a version; nil fields are left unchanged.
type AndroidVersionMeta struct {
	Notes     *string
	Changelog *string
	Tags      []string // nil: unchanged
	Channel   *string
	Origin    *string
	ParentID  *string // empty string clears
	SourceURL *string
	PatchInfo json.RawMessage
	Pinned    *bool
}

// AndroidAppMeta is the hand-written part of an app; nil fields are left unchanged.
type AndroidAppMeta struct {
	LabelOverride  *string
	Notes          *string
	Tags           []string // nil: unchanged
	SignerPolicy   *string
	UpstreamKind   *string
	UpstreamRef    *string
	UpstreamLatest *string
}

// AndroidIcon is an icon (or adaptive-icon layer) extracted from a version.
type AndroidIcon struct {
	ID           string    `json:"id"`
	VersionID    string    `json:"version_id"`
	Role         string    `json:"role"`
	Density      string    `json:"density"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Mime         string    `json:"mime"`
	SourcePath   string    `json:"source_path"`
	ResourceName string    `json:"resource_name,omitempty"`
	Color        string    `json:"color,omitempty"`
	Adaptive     bool      `json:"adaptive"`
	Renderable   bool      `json:"renderable"`
	BlobID       string    `json:"-"`
	SHA256       string    `json:"sha256,omitempty"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

// AndroidFile is a file attached to a version.
type AndroidFile struct {
	ID        string    `json:"id"`
	VersionID string    `json:"version_id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	BlobID    string    `json:"-"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// AndroidSigner is a known signing key (public data only).
type AndroidSigner struct {
	ID           string          `json:"id"`
	RepositoryID string          `json:"repository_id"`
	SHA256       string          `json:"sha256"`
	Label        string          `json:"label"`
	Trust        string          `json:"trust"`
	AppID        string          `json:"app_id,omitempty"`
	Notes        string          `json:"notes"`
	Certificate  json.RawMessage `json:"certificate,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// AndroidTagCount is a tag with the number of apps and versions that carry it.
type AndroidTagCount struct {
	Tag      string `json:"tag"`
	Apps     int    `json:"apps"`
	Versions int    `json:"versions"`
}

// AndroidSignerUse summarises one key used by the versions of an app.
type AndroidSignerUse struct {
	SHA256   string    `json:"sha256"`
	Versions int       `json:"versions"`
	First    time.Time `json:"first"`
	Last     time.Time `json:"last"`
	Origins  []string  `json:"origins"`
}
