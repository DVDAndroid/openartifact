// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request, ApiError } from './client'
import { pagedResponse, emptyPaged, type PagedResult } from './transform'
import type { AuditEntry } from './generated/types.gen'

export type { AuditEntry }

export interface AuditFilters {
  page?: number
  limit?: number
  actor?: string
  action?: string
  from?: string
  to?: string
  cursor?: string
}

export async function listAudit(filters?: AuditFilters): Promise<PagedResult<AuditEntry>> {
  const qs = new URLSearchParams()
  if (filters?.page != null) qs.set('page', String(filters.page))
  if (filters?.limit != null) qs.set('per_page', String(filters.limit))
  if (filters?.actor) qs.set('actor', filters.actor)
  if (filters?.action) qs.set('action', filters.action)
  if (filters?.from) qs.set('from', filters.from)
  if (filters?.to) qs.set('to', filters.to)
  if (filters?.cursor) qs.set('cursor', filters.cursor)
  const query = qs.toString() ? `?${qs}` : ''
  try {
    const raw = await request<Record<string, unknown>>(`/api/v1/admin/audit${query}`)
    const result = pagedResponse<AuditEntry>(raw)
    return result
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 501)) return emptyPaged<AuditEntry>()
    throw err
  }
}
