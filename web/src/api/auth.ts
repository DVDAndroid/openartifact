// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request, ApiError } from './client'
import type { User } from '../stores/session'

export interface SSOProvider {
  slug: string
  kind: 'oidc' | 'saml'
  display_name: string
  enabled: boolean
}

export function login(credentials: { username: string; password: string }): Promise<User> {
  return request<User>('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(credentials),
  })
}

export function logout(): Promise<void> {
  return request<void>('/api/v1/auth/logout', { method: 'POST' })
}

export function me(): Promise<User> {
  return request<User>('/api/v1/auth/me')
}

export function changePassword(body: { current_password: string; new_password: string }): Promise<void> {
  return request<void>('/api/v1/auth/change-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function listSSOProviders(): Promise<SSOProvider[]> {
  try {
    const res = await request<{ providers: SSOProvider[] }>('/api/v1/auth/sso/providers')
    return res?.providers ?? []
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return []
    }
    throw err
  }
}
