// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request } from './client'
import type { Repository } from './generated/types.gen'

export type { Repository }

export interface PackageItem {
  name: string
  version: string
  size?: number
  sha256?: string
  format: string
  created_at?: string
}

interface ListRepositoriesResult {
  total: number
  repositories: Repository[]
}

interface ListPackagesResult {
  total: number
  packages: PackageItem[]
}

export function listRepositories(params?: { page?: number; limit?: number; format?: string }): Promise<ListRepositoriesResult> {
  const qs = new URLSearchParams()
  if (params?.page != null) qs.set('page', String(params.page))
  if (params?.limit != null) qs.set('limit', String(params.limit))
  if (params?.format) qs.set('format', params.format)
  const query = qs.toString() ? `?${qs}` : ''
  return request<ListRepositoriesResult>(`/api/v1/repositories${query}`)
}

export function getRepository(name: string): Promise<Repository> {
  return request<Repository>(`/api/v1/repository/${encodeURIComponent(name)}`)
}

export function createRepository(body: Partial<Repository>): Promise<Repository> {
  return request<Repository>('/api/v1/repositories', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export function updateRepository(name: string, body: Partial<Repository>): Promise<Repository> {
  return request<Repository>(`/api/v1/repository/${encodeURIComponent(name)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export function deleteRepository(name: string): Promise<void> {
  return request<void>(`/api/v1/repository/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  })
}

export function listPackages(name: string, params?: { page?: number; limit?: number; format?: string }): Promise<ListPackagesResult> {
  const qs = new URLSearchParams()
  if (params?.page != null) qs.set('page', String(params.page))
  if (params?.limit != null) qs.set('limit', String(params.limit))
  if (params?.format) qs.set('format', params.format)
  const query = qs.toString() ? `?${qs}` : ''
  return request<ListPackagesResult>(`/api/v1/repositories/${encodeURIComponent(name)}/packages${query}`)
}

export interface ComponentItem {
  name: string
  version: string
  format: string
  size?: number
  sha256?: string
  created_at?: string
  path?: string
}

interface ListComponentsResult {
  total: number
  components: ComponentItem[]
}

// ── SBOM ──────────────────────────────────────────────────────────────────────

export interface SbomListItem {
  id: string
  artifact_name?: string
  artifact_version?: string
  created_at?: string
  format?: string
}

export interface SbomComponent {
  'bom-ref'?: string
  type?: string
  name: string
  version?: string
  purl?: string
  licenses?: Array<{ license?: { id?: string; name?: string } }>
  hashes?: Array<{ alg?: string; content?: string }>
}

export interface SbomData {
  id?: string
  bomFormat?: string
  specVersion?: string
  metadata?: {
    component?: {
      'bom-ref'?: string
      name: string
      version?: string
      type?: string
    }
  }
  components?: SbomComponent[]
  dependencies?: Array<{ ref: string; dependsOn?: string[] }>
}

export function listSboms(repoName: string): Promise<SbomListItem[]> {
  return request<SbomListItem[]>(`/api/v1/repositories/${encodeURIComponent(repoName)}/sbom`)
}

export function getSbom(repoName: string, id: string): Promise<SbomData> {
  return request<SbomData>(`/api/v1/repositories/${encodeURIComponent(repoName)}/sbom/${encodeURIComponent(id)}`)
}

export function deletePackage(repoName: string, format: string, name: string): Promise<void> {
  return request<void>(
    `/api/v1/repositories/${encodeURIComponent(repoName)}/packages/${encodeURIComponent(format)}/${encodeURIComponent(name)}`,
    { method: 'DELETE' },
  )
}

// ── Components ────────────────────────────────────────────────────────────────

export async function listComponents(
  name: string,
  params?: { page?: number; limit?: number; prefix?: string },
): Promise<{ total: number; items: ComponentItem[] }> {
  const qs = new URLSearchParams()
  if (params?.page != null) qs.set('page', String(params.page))
  if (params?.limit != null) qs.set('limit', String(params.limit))
  if (params?.prefix) qs.set('prefix', params.prefix)
  const query = qs.toString() ? `?${qs}` : ''

  try {
    const res = await request<ListComponentsResult>(
      `/api/v1/repositories/${encodeURIComponent(name)}/components${query}`,
    )
    return { total: res.total, items: res.components }
  } catch {
    // Fall back to packages endpoint
    const res = await request<ListPackagesResult>(
      `/api/v1/repositories/${encodeURIComponent(name)}/packages${query}`,
    )
    return {
      total: res.total,
      items: res.packages.map((p) => ({
        name: p.name,
        version: p.version,
        format: p.format,
        size: p.size,
        sha256: p.sha256,
        created_at: p.created_at,
      })),
    }
  }
}
