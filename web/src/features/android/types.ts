// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Wire types of the android management API (internal/android, pkg/apk). They
// are written by hand rather than generated: the endpoints are described by
// an OpenAPI fragment merged at start-up, not by internal/api/openapi.json.

export type Origin = 'built' | 'patched' | 'original' | 'unknown'
export type Trust = 'mine' | 'original' | 'third_party' | 'untrusted'
export type SignerPolicy = 'allow' | 'warn' | 'reject'
export type AppSort = 'name' | 'updated' | 'versions' | 'size'

export const ORIGINS: readonly Origin[] = ['built', 'patched', 'original', 'unknown']
export const TRUST_LEVELS: readonly Trust[] = ['mine', 'original', 'third_party', 'untrusted']
export const SIGNER_POLICIES: readonly SignerPolicy[] = ['allow', 'warn', 'reject']
export const FILE_KINDS = [
  'mapping', 'symbols', 'patch_bundle', 'patch_options', 'source', 'screenshot', 'original_apk', 'other',
] as const
export type FileKind = (typeof FILE_KINDS)[number]

export interface Page<T> {
  items: T[]
  total: number
  page: number
  limit: number
}

export interface AndroidApp {
  id: string
  repository_id: string
  package_name: string
  label_override?: string
  notes: string
  tags: string[]
  signer_policy: SignerPolicy
  upstream_kind?: string
  upstream_ref?: string
  upstream_latest?: string
  created_at: string
  updated_at: string
}

export interface AndroidAppSummary extends AndroidApp {
  label: string
  version_count: number
  origins: Origin[]
  signers: string[]
  total_size_bytes: number
  latest_version_id: string
  latest_version_name: string
  latest_version_code: number
  latest_signer_sha256: string
  latest_origin: Origin
  latest_created_at: string
  icon_url?: string
}

export interface CertInfo {
  sha256: string
  sha1: string
  md5: string
  public_key_sha256: string
  subject: string
  issuer: string
  serial: string
  not_before: string
  not_after: string
  public_key_algorithm: string
  public_key_size: number
  signature_algorithm: string
  self_signed: boolean
  flags?: string[]
}

export interface LineageNode {
  certificate: CertInfo | null
  flags: number
  signature_algorithm?: string
}

export interface SchemeSigner {
  certificate: CertInfo | null
  chain?: CertInfo[]
  min_sdk?: number
  max_sdk?: number
  digest_algorithms?: string[]
  signature_algorithms?: string[]
  verified: boolean
  errors?: string[]
}

export interface SchemeResult {
  scheme: string
  verified: boolean
  signers: SchemeSigner[]
  lineage?: LineageNode[]
  errors?: string[]
  warnings?: string[]
  rotation_min_sdk?: number
}

export interface SignatureInfo {
  schemes: string[]
  verified: boolean
  results: SchemeResult[]
  signer?: CertInfo
  signers: CertInfo[]
  lineage?: LineageNode[]
  blocks?: string[]
  flags?: string[]
  errors?: string[]
}

export interface Permission {
  name: string
  max_sdk?: number
  protection_level?: string
  sdk23?: boolean
}

export interface Feature {
  name?: string
  required: boolean
  gles_version?: string
}

export interface IntentFilter {
  actions?: string[]
  categories?: string[]
  data?: string[]
}

export interface Component {
  kind: string
  name: string
  exported?: boolean
  enabled?: boolean
  permission?: string
  authorities?: string
  launcher?: boolean
  intent_filters?: IntentFilter[]
}

export interface AppFlags {
  debuggable: boolean
  test_only?: boolean
  allow_backup?: boolean
  uses_cleartext_traffic?: boolean
  network_security_config?: string
  extract_native_libs?: boolean
  has_code?: boolean
  name?: string
  app_component_factory?: string
}

export interface Manifest {
  package: string
  version_code: number
  version_name: string
  min_sdk: number
  target_sdk: number
  max_sdk?: number
  compile_sdk?: number
  compile_sdk_codename?: string
  shared_user_id?: string
  install_location?: string
  is_split?: boolean
  split?: string
  application: AppFlags
  permissions: Permission[]
  declared_permissions?: Permission[]
  features: Feature[]
  components: Component[]
  queries?: string[]
  uses_libraries?: string[]
}

export interface NativeLib {
  abi: string
  name: string
  path: string
  size: number
  stored: boolean
  aligned_16k: boolean
  class?: string
  machine?: string
  soname?: string
  needed?: string[]
  build_id?: string
  stripped: boolean
  load_align?: number
  error?: string
}

export interface Dependency {
  group: string
  artifact: string
  version: string
  source: string
}

export interface BuildInfo {
  agp_version?: string
  app_metadata_version?: string
  kotlin?: boolean
  compose?: boolean
  dex_files: number
}

export interface ZipChecks {
  aligned_4: boolean
  unaligned?: string[]
  native_aligned_16k: boolean
  native_compressed: boolean
  resources_arsc_stored: boolean
  resources_arsc_aligned: boolean
  prefix_bytes?: number
  duplicate_entries?: string[]
  problems?: string[]
  entry_count: number
  uncompressed_total: number
}

export interface ZipEntry {
  name: string
  method: number
  crc32: number
  compressed_size: number
  size: number
}

export interface AndroidVersion {
  id: string
  repository_id: string
  app_id: string
  package_name: string
  version_code: number
  version_name: string
  file_kind: string
  sha256: string
  size_bytes: number
  filename: string
  signer_sha256: string
  abis: string[]
  variant_key: string
  label: string
  labels?: Record<string, string>
  launcher_label?: string
  min_sdk: number
  target_sdk: number
  compile_sdk?: number
  max_sdk?: number
  debuggable: boolean
  locales: string[]
  frameworks: string[]
  sig_schemes: string[]
  sig_verified: boolean
  content_digest: string
  page_size_16k: boolean
  origin: Origin
  parent_id?: string
  patch_info?: Record<string, unknown>
  source_url?: string
  original_sha256?: string
  changelog: string
  notes: string
  tags: string[]
  channel: string
  pinned: boolean
  download_count: number
  created_by?: string
  created_at: string
  updated_at: string
  download_url: string
  icon_url: string
  // Detail only.
  manifest?: Manifest
  signature?: SignatureInfo
  native_libs?: NativeLib[]
  dependencies?: Dependency[]
  build_info?: BuildInfo
  zip_checks?: ZipChecks
  entries?: ZipEntry[]
  warnings?: string[]
}

export interface KeyView {
  sha256: string
  known: boolean
  trust?: Trust
  label?: string
}

export interface ParentView {
  id: string
  version_name: string
  version_code: number
  origin: Origin
  signer_sha256: string
  same_signer: boolean
  same_content: boolean
}

export interface AndroidIcon {
  id: string
  version_id: string
  role: string
  density: string
  width: number
  height: number
  mime: string
  source_path: string
  resource_name?: string
  color?: string
  adaptive: boolean
  renderable: boolean
  sha256?: string
  size_bytes: number
  created_at: string
  url?: string
}

export interface AndroidFile {
  id: string
  version_id: string
  kind: FileKind
  name: string
  sha256: string
  size_bytes: number
  created_at: string
}

export interface VersionDetail {
  version: AndroidVersion
  signer: KeyView | null
  parent: ParentView | null
  icons: AndroidIcon[]
  files: AndroidFile[]
}

export interface SignerUse {
  sha256: string
  versions: number
  first: string
  last: string
  origins: Origin[]
  known: boolean
  trust?: Trust
  label?: string
}

export interface AppDetail {
  app: AndroidApp
  signers: SignerUse[]
}

export interface AndroidSigner {
  id: string
  repository_id: string
  sha256: string
  label: string
  trust: Trust
  app_id?: string
  notes: string
  certificate?: Partial<CertInfo>
  created_at: string
  updated_at: string
}

export interface TagCount {
  tag: string
  apps: number
  versions: number
}

export interface InspectResult {
  info: {
    manifest: Manifest
    label: string
    labels: Record<string, string>
    launcher_label?: string
    icons: Array<Omit<AndroidIcon, 'id' | 'version_id' | 'created_at' | 'size_bytes'> & { size: number }>
    signature: SignatureInfo
    content_digest: string
    abis: string[]
    native_libs: NativeLib[]
    page_size_16k: boolean
    frameworks: string[]
    warnings: string[]
  }
  sha256: string
  size_bytes: number
  filename: string
  duplicate: string | null
  previous: AndroidVersion | null
}

export interface UploadResult {
  id: string
  package_name: string
  version_code: number
  version_name: string
  sha256: string
  size_bytes: number
  signer_sha256: string
  origin: Origin
  parent_id?: string
  original_id?: string
  filename: string
  warnings?: string[]
}

export interface UploadOptions {
  origin: Origin
  original?: File | null
  parent?: string
  tags?: string[]
  channel?: string
  changelog?: string
  notes?: string
  source_url?: string
  patch_info?: string
  accept_new_signer?: boolean
  no_original?: boolean
}

export interface AppFilter {
  q?: string
  tags?: string[]
  origin?: Origin | ''
  signer?: string
  sort?: AppSort
  order?: 'asc' | 'desc'
  page?: number
  limit?: number
}

export interface AppMetaUpdate {
  label_override?: string
  notes?: string
  tags?: string[]
  signer_policy?: SignerPolicy
}

export interface VersionMetaUpdate {
  notes?: string
  changelog?: string
  tags?: string[]
  channel?: string
  origin?: Origin
  parent_id?: string
  source_url?: string
  patch_info?: Record<string, unknown>
  pinned?: boolean
}

export interface SignerCreate {
  sha256?: string
  certificate?: string
  trust: Trust
  label?: string
  notes?: string
  app?: string
}
