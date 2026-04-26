// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request, ApiError } from './client'
import { pagedResponse, emptyPaged, type PagedResult } from './transform'
import type { StorageBackend } from './generated/types.gen'

export type { StorageBackend }

export async function listStorageBackends(): Promise<PagedResult<StorageBackend>> {
  try {
    const raw = await request<Record<string, unknown>>('/api/v1/admin/storage')
    return pagedResponse<StorageBackend>(raw)
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 501)) return emptyPaged<StorageBackend>()
    throw err
  }
}

export async function getStorageBackend(name: string): Promise<StorageBackend> {
  return request<StorageBackend>(`/api/v1/admin/storage/${encodeURIComponent(name)}`)
}

export async function createStorageBackend(body: {
  name: string
  kind: string
  is_default?: boolean
  config: Record<string, unknown>
}): Promise<StorageBackend> {
  return request<StorageBackend>('/api/v1/admin/storage', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function updateStorageBackend(
  name: string,
  body: Partial<{ name: string; kind: string; is_default: boolean; config: Record<string, unknown> }>,
): Promise<StorageBackend> {
  return request<StorageBackend>(`/api/v1/admin/storage/${encodeURIComponent(name)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function deleteStorageBackend(name: string): Promise<void> {
  await request<void>(`/api/v1/admin/storage/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  })
}

export async function testStorageBackend(name: string): Promise<{ ok: boolean; message?: string }> {
  try {
    return await request<{ ok: boolean; message?: string }>(
      `/api/v1/admin/storage/${encodeURIComponent(name)}/test`,
      { method: 'POST' },
    )
  } catch (err) {
    if (err instanceof ApiError) return { ok: false, message: err.message }
    throw err
  }
}
