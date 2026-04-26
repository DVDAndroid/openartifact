// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request, ApiError } from './client'
import { pagedResponse, emptyPaged, type PagedResult } from './transform'
import type { User, Group, PermissionTarget, SystemInfo } from './generated/types.gen'

export type UserItem = User
export type GroupItem = Group
export type PermissionItem = PermissionTarget
export type { SystemInfo }

async function paged<T>(qs: URLSearchParams, path: string): Promise<PagedResult<T>> {
  const query = qs.toString() ? `?${qs}` : ''
  try {
    const raw = await request<Record<string, unknown>>(`${path}${query}`)
    return pagedResponse<T>(raw)
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 501)) {
      return emptyPaged<T>()
    }
    throw err
  }
}

export interface ListParams {
  page?: number
  limit?: number
  q?: string
}

function buildQs(p?: ListParams): URLSearchParams {
  const qs = new URLSearchParams()
  if (p?.page != null) qs.set('page', String(p.page))
  if (p?.limit != null) qs.set('per_page', String(p.limit))
  if (p?.q) qs.set('q', p.q)
  return qs
}

// Users
export const listUsers = (p?: ListParams) => paged<UserItem>(buildQs(p), '/api/v1/users')
export const createUser = (body: { username: string; email?: string; full_name?: string; password: string; force_change_password?: boolean }) =>
  request<UserItem>('/api/v1/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
export const updateUser = (id: string | number, body: Partial<User>) =>
  request<UserItem>(`/api/v1/users/${encodeURIComponent(String(id))}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
export const deleteUser = (id: string | number) =>
  request<void>(`/api/v1/users/${encodeURIComponent(String(id))}`, { method: 'DELETE' })
export const setUserDisabled = (id: string | number, disabled: boolean) =>
  request<UserItem>(`/api/v1/users/${encodeURIComponent(String(id))}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ disabled }),
  })
export const forceUserPasswordReset = (id: string | number) =>
  request<UserItem>(`/api/v1/users/${encodeURIComponent(String(id))}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ must_change_password: true }),
  })

// Groups
export const listGroups = (p?: ListParams) => paged<GroupItem>(buildQs(p), '/api/v1/groups')
export const createGroup = (body: { name: string }) =>
  request<GroupItem>('/api/v1/groups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
export const renameGroup = (id: string | number, name: string) =>
  request<GroupItem>(`/api/v1/groups/${encodeURIComponent(String(id))}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
export const deleteGroup = (id: string | number) =>
  request<void>(`/api/v1/groups/${encodeURIComponent(String(id))}`, { method: 'DELETE' })
export const updateGroupMembers = (id: string | number, members: string[]) =>
  request<GroupItem>(`/api/v1/groups/${encodeURIComponent(String(id))}/members`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ members }),
  })

// Permissions
export const listPermissions = (p?: ListParams) => paged<PermissionItem>(buildQs(p), '/api/v1/permission-targets')
export const grantPermission = (body: { principal: string; scope: string; role: string }) =>
  request<PermissionItem>('/api/v1/permission-targets', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
export const revokePermission = (id: string | number) =>
  request<void>(`/api/v1/permission-targets/${encodeURIComponent(String(id))}`, { method: 'DELETE' })

// System
export async function getSystemInfo(): Promise<SystemInfo | null> {
  try {
    return await request<SystemInfo>('/api/v1/system/info')
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 501)) {
      return null
    }
    throw err
  }
}

// SSO config
export interface SSOConfigInput {
  slug: string
  kind: 'oidc' | 'saml'
  display_name: string
  enabled: boolean
  config: Record<string, unknown>
}

export async function saveSSOProvider(body: SSOConfigInput): Promise<void> {
  return request<void>('/api/v1/admin/sso-providers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function testSSOProvider(slug: string): Promise<{ ok: boolean; message?: string }> {
  try {
    return await request<{ ok: boolean; message?: string }>(
      `/api/v1/admin/sso-providers/${encodeURIComponent(slug)}/test`,
      { method: 'POST' },
    )
  } catch (err) {
    if (err instanceof ApiError) return { ok: false, message: err.message }
    throw err
  }
}
