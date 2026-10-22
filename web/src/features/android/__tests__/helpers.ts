// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { AndroidAppSummary, AndroidSigner, AndroidVersion, CertInfo, SignatureInfo, VersionDetail } from '../types'

export const SIGNER = 'cd'.repeat(32)

export function cert(sha256 = SIGNER): CertInfo {
  return {
    sha256, sha1: '11'.repeat(20), md5: '22'.repeat(16), public_key_sha256: '33'.repeat(32),
    subject: 'CN=Me', issuer: 'CN=Me', serial: '1', not_before: '2020-01-01T00:00:00Z', not_after: '2050-01-01T00:00:00Z',
    public_key_algorithm: 'RSA', public_key_size: 4096, signature_algorithm: 'SHA256withRSA', self_signed: true,
  }
}

export function signature(sha256 = SIGNER): SignatureInfo {
  return {
    schemes: ['v2', 'v3'], verified: true, signer: cert(sha256), signers: [cert(sha256)],
    results: [
      { scheme: 'v2', verified: true, signers: [{ certificate: cert(sha256), verified: true }] },
      { scheme: 'v3', verified: true, signers: [{ certificate: cert(sha256), verified: true }] },
    ],
  }
}

export function version(over: Partial<AndroidVersion> = {}): AndroidVersion {
  return {
    id: '11111111-1111-1111-1111-111111111111', repository_id: 'r', app_id: 'a', package_name: 'org.example.app',
    version_code: 7, version_name: '1.2.3', file_kind: 'apk', sha256: 'ef'.repeat(32), size_bytes: 2048,
    filename: 'org.example.app-7-cdcdcdcd.apk', signer_sha256: SIGNER, abis: ['arm64-v8a'], variant_key: 'k',
    label: 'Example', min_sdk: 24, target_sdk: 34, debuggable: false, locales: [], frameworks: [],
    sig_schemes: ['v2', 'v3'], sig_verified: true, content_digest: 'dd', page_size_16k: true, origin: 'built',
    changelog: '', notes: '', tags: [], channel: 'stable', pinned: false, download_count: 3,
    created_at: '2025-01-02T03:04:05Z', updated_at: '2025-01-02T03:04:05Z',
    download_url: 'https://oa.example.com/repos/phone/apps/org.example.app/7/org.example.app-7-cdcdcdcd.apk',
    icon_url: '/api/v1/repositories/phone/apks/versions/1/icon?size=96',
    ...over,
  }
}

export function detail(over: Partial<AndroidVersion> = {}, extra: Partial<VersionDetail> = {}): VersionDetail {
  return {
    version: version({ signature: signature(), warnings: [], ...over }),
    signer: { sha256: SIGNER, known: true, trust: 'mine', label: 'release key' },
    parent: null, icons: [], files: [], ...extra,
  }
}

export function app(over: Partial<AndroidAppSummary> = {}): AndroidAppSummary {
  return {
    id: 'a', repository_id: 'r', package_name: 'org.example.app', notes: '', tags: ['device:pixel'], signer_policy: 'warn',
    created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-02T00:00:00Z', label: 'Example', version_count: 2,
    origins: ['built', 'original'], signers: [SIGNER], total_size_bytes: 4096, latest_version_id: '1',
    latest_version_name: '1.2.3', latest_version_code: 7, latest_signer_sha256: SIGNER, latest_origin: 'built',
    latest_created_at: '2025-01-02T03:04:05Z', icon_url: '/icon.png', ...over,
  }
}

export function knownKey(over: Partial<AndroidSigner> = {}): AndroidSigner {
  return {
    id: 'k1', repository_id: 'r', sha256: SIGNER, label: 'release key', trust: 'mine', notes: '',
    created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z', ...over,
  }
}

type Handler = (url: string, init: RequestInit) => unknown | Response

/**
 * Replaces fetch with a router over exact-path handlers. Handlers return the
 * payload (wrapped in the {success, data} envelope) or a ready Response.
 */
export function mockApi(routes: Record<string, Handler>): { calls: Array<{ url: string; init: RequestInit }> } {
  const calls: Array<{ url: string; init: RequestInit }> = []
  globalThis.fetch = (async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const url = String(input)
    calls.push({ url, init })
    const key = `${init.method ?? 'GET'} ${url.split('?')[0]}`
    const handler = routes[key]
    if (!handler) return new Response(JSON.stringify({ error: `no route for ${key}`, code: 'NOT_FOUND' }), { status: 404 })
    const out = handler(url, init)
    if (out instanceof Response) return out
    return new Response(JSON.stringify({ success: true, data: out }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }) as typeof fetch
  return { calls }
}
