// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { ApiError, csrfHeader, request } from '../../api/client'
import type {
  AndroidFile,
  AndroidSigner,
  AndroidVersion,
  AppDetail,
  AppFilter,
  AppMetaUpdate,
  AndroidAppSummary,
  FileKind,
  InspectResult,
  Page,
  SignerCreate,
  TagCount,
  UploadOptions,
  UploadResult,
  VersionDetail,
  VersionMetaUpdate,
  AndroidApp,
} from './types'

// Every android endpoint wraps its payload in {success, data}.
interface Envelope<T> {
  success: boolean
  data: T
}

const JSON_HEADERS = { 'Content-Type': 'application/json' }

function base(repo: string): string {
  return `/api/v1/repositories/${encodeURIComponent(repo)}/apks`
}

async function data<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await request<Envelope<T>>(path, init)
  return res.data
}

function json(method: string, body: unknown): RequestInit {
  return { method, headers: JSON_HEADERS, body: JSON.stringify(body) }
}

function query(params: Record<string, string | number | undefined | string[]>): string {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '' || (Array.isArray(v) && v.length === 0)) continue
    qs.set(k, Array.isArray(v) ? v.join(',') : String(v))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

export function listApps(repo: string, f: AppFilter = {}): Promise<Page<AndroidAppSummary>> {
  return data(`${base(repo)}/apps${query({
    q: f.q, tag: f.tags, origin: f.origin || undefined, signer: f.signer,
    sort: f.sort, order: f.order, page: f.page, limit: f.limit,
  })}`)
}

export function getApp(repo: string, pkg: string): Promise<AppDetail> {
  return data(`${base(repo)}/apps/${encodeURIComponent(pkg)}`)
}

export function updateApp(repo: string, pkg: string, body: AppMetaUpdate): Promise<AndroidApp> {
  return data(`${base(repo)}/apps/${encodeURIComponent(pkg)}/meta`, json('PUT', body))
}

export function listVersions(
  repo: string,
  pkg: string,
  f: { origin?: string; signer?: string; tags?: string[]; page?: number; limit?: number } = {},
): Promise<Page<AndroidVersion>> {
  return data(`${base(repo)}/apps/${encodeURIComponent(pkg)}/versions${query({
    origin: f.origin, signer: f.signer, tag: f.tags, page: f.page, limit: f.limit,
  })}`)
}

export function getVersion(repo: string, id: string): Promise<VersionDetail> {
  return data(`${base(repo)}/versions/${encodeURIComponent(id)}`)
}

export function updateVersion(repo: string, id: string, body: VersionMetaUpdate): Promise<AndroidVersion> {
  return data(`${base(repo)}/versions/${encodeURIComponent(id)}/meta`, json('PUT', body))
}

export async function deleteVersion(repo: string, id: string): Promise<void> {
  await request(`${base(repo)}/versions/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

/** Decoded AndroidManifest.xml, served as plain text. */
export async function getManifestXml(repo: string, id: string): Promise<string> {
  const res = await fetch(`${base(repo)}/versions/${encodeURIComponent(id)}/manifest.xml`, { credentials: 'include' })
  if (!res.ok) throw new ApiError(`HTTP ${res.status}`, res.status)
  return res.text()
}

export function versionIconUrl(repo: string, id: string, size = 96): string {
  return `${base(repo)}/versions/${encodeURIComponent(id)}/icon?size=${size}`
}

export function listFiles(repo: string, id: string): Promise<AndroidFile[]> {
  return data(`${base(repo)}/versions/${encodeURIComponent(id)}/files`)
}

export function fileUrl(repo: string, id: string, fileId: string): string {
  return `${base(repo)}/versions/${encodeURIComponent(id)}/files/${encodeURIComponent(fileId)}`
}

export async function deleteFile(repo: string, id: string, fileId: string): Promise<void> {
  await request(fileUrl(repo, id, fileId), { method: 'DELETE' })
}

export function attachFile(repo: string, id: string, kind: FileKind, file: File): Promise<AndroidFile> {
  const form = new FormData()
  form.append('kind', kind)
  form.append('file', file, file.name)
  return data(`${base(repo)}/versions/${encodeURIComponent(id)}/files`, { method: 'POST', body: form })
}

export function listSigners(repo: string, app?: string): Promise<AndroidSigner[]> {
  return data(`${base(repo)}/signers${query({ app })}`)
}

export function createSigner(repo: string, body: SignerCreate): Promise<AndroidSigner> {
  return data(`${base(repo)}/signers`, json('POST', body))
}

export function updateSigner(
  repo: string,
  id: string,
  body: { label?: string; trust?: string; notes?: string },
): Promise<AndroidSigner> {
  return data(`${base(repo)}/signers/${encodeURIComponent(id)}`, json('PUT', body))
}

export async function deleteSigner(repo: string, id: string): Promise<void> {
  await request(`${base(repo)}/signers/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function listTags(repo: string): Promise<TagCount[]> {
  return data(`${base(repo)}/tags`)
}

// ─── uploads ────────────────────────────────────────────────────────────────

function apkForm(file: File, extra: Record<string, string | Blob | undefined> = {}): FormData {
  const form = new FormData()
  form.append('file', file, file.name)
  for (const [k, v] of Object.entries(extra)) if (v !== undefined && v !== '') form.append(k, v)
  return form
}

/** Analyses a file without storing it. */
export function inspectApk(repo: string, file: File): Promise<InspectResult> {
  return data(`${base(repo)}/inspect`, { method: 'POST', body: apkForm(file) })
}

function uploadFields(o: UploadOptions): Record<string, string | Blob | undefined> {
  return {
    origin: o.origin,
    original: o.original ?? undefined,
    parent: o.parent,
    tags: o.tags?.join(','),
    channel: o.channel,
    changelog: o.changelog,
    notes: o.notes,
    source_url: o.source_url,
    patch_info: o.patch_info,
    accept_new_signer: o.accept_new_signer ? 'true' : undefined,
    no_original: o.no_original ? 'true' : undefined,
  }
}

export interface UploadHandle {
  promise: Promise<UploadResult>
  abort: () => void
}

/**
 * Stores a file. XHR instead of fetch so the progress of a large APK can be
 * shown; the CSRF and session handling mirror request().
 */
export function uploadApk(
  repo: string,
  file: File,
  options: UploadOptions,
  onProgress: (loaded: number, total: number) => void,
): UploadHandle {
  const xhr = new XMLHttpRequest()
  const promise = new Promise<UploadResult>((resolve, reject) => {
    xhr.open('POST', base(repo))
    xhr.withCredentials = true
    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest')
    for (const [k, v] of Object.entries(csrfHeader())) xhr.setRequestHeader(k, v)
    xhr.upload.addEventListener('progress', (ev) => {
      if (ev.lengthComputable) onProgress(ev.loaded, ev.total)
    })
    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve((JSON.parse(xhr.responseText) as Envelope<UploadResult>).data)
        } catch {
          reject(new ApiError('invalid response', xhr.status))
        }
        return
      }
      reject(parseXhrError(xhr))
    })
    xhr.addEventListener('error', () => reject(new ApiError('network error', 0)))
    xhr.addEventListener('abort', () => reject(new ApiError('aborted', 0, 'aborted')))
    xhr.send(apkForm(file, uploadFields(options)))
  })
  return { promise, abort: () => xhr.abort() }
}

/** An upload failure; a 409 carries what conflicted (`duplicate`, or the rejected `signer_sha256`). */
export class UploadError extends ApiError {
  details: Record<string, unknown>

  constructor(message: string, status: number, code?: string, details: Record<string, unknown> = {}) {
    super(message, status, code)
    this.name = 'UploadError'
    this.details = details
  }
}

function parseXhrError(xhr: XMLHttpRequest): UploadError {
  let message = `HTTP ${xhr.status}`
  let code: string | undefined
  let details: Record<string, unknown> | undefined
  try {
    const body = JSON.parse(xhr.responseText) as { error?: string; code?: string; details?: Record<string, unknown> }
    if (body.error) message = body.error
    if (body.code) code = body.code
    details = body.details
  } catch {
    // non-JSON body: keep the HTTP status message
  }
  return new UploadError(message, xhr.status, code, details)
}
