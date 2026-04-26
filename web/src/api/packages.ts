// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request, ApiError, csrfHeader } from './client'

export interface PackageDetail {
  name: string
  version: string
  format: string
  repository: string
  size?: number
  sha256?: string
  filename?: string
  description?: string
  dependencies?: PackageDependency[]
  files?: PackageFile[]
  download_count?: number
  download_history?: DownloadStat[]
  created_at?: string
}

export interface PackageDependency {
  name: string
  version_constraint?: string
  in_repo?: boolean
  format?: string
  repository?: string
}

export interface PackageFile {
  path: string
  size?: number
}

export interface DownloadStat {
  date: string
  count: number
}

export async function getPackageDetail(
  repo: string,
  format: string,
  pkg: string,
  version?: string,
): Promise<PackageDetail | null> {
  const qs = new URLSearchParams()
  if (version) qs.set('version', version)
  const query = qs.toString() ? `?${qs}` : ''
  try {
    return await request<PackageDetail>(
      `/api/v1/repositories/${encodeURIComponent(repo)}/packages/${encodeURIComponent(format)}/${encodeURIComponent(pkg)}${query}`,
    )
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 501)) {
      return null
    }
    throw err
  }
}

export interface UploadOptions {
  repo: string
  format?: string
  file: File
  onProgress?: (loaded: number, total: number) => void
  signal?: AbortSignal
}

// `uploadPackage` is the simple Promise-based wrapper. Components needing
// abort/progress at scale should use the `useUpload` composable.
export function uploadPackage(opts: UploadOptions): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const url = `/api/v1/repositories/${encodeURIComponent(opts.repo)}/packages${
      opts.format ? `?format=${encodeURIComponent(opts.format)}` : ''
    }`
    xhr.open('POST', url)
    xhr.withCredentials = true
    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest')
    for (const [k, v] of Object.entries(csrfHeader())) xhr.setRequestHeader(k, v)
    xhr.upload.addEventListener('progress', (ev) => {
      if (ev.lengthComputable) opts.onProgress?.(ev.loaded, ev.total)
    })
    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve()
      else reject(new ApiError(xhr.responseText || `HTTP ${xhr.status}`, xhr.status))
    })
    xhr.addEventListener('error', () => reject(new ApiError('network error', 0)))
    xhr.addEventListener('abort', () => reject(new ApiError('aborted', 0)))
    if (opts.signal) {
      opts.signal.addEventListener('abort', () => xhr.abort(), { once: true })
    }
    const form = new FormData()
    form.append('file', opts.file, opts.file.name)
    xhr.send(form)
  })
}
