// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

export class ApiError extends Error {
  status: number
  code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

function csrfToken(): string | undefined {
  for (const cookie of document.cookie.split(';')) {
    const [name, value] = cookie.trim().split('=', 2)
    if (name === 'oa_csrf') return value
  }
}

export function csrfHeader(): Record<string, string> {
  const token = csrfToken()
  return token ? { 'X-CSRF-Token': token } : {}
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('X-Requested-With', 'XMLHttpRequest')
  for (const [name, value] of Object.entries(csrfHeader())) headers.set(name, value)

  const res = await fetch(path, {
    ...init,
    headers,
    credentials: 'include',
  })

  // The self-service login endpoint returns 401 for bad credentials, not an
  // expired session — let it propagate as a thrown ApiError so callers (e.g.
  // LoginPage) can show the real error instead of being silently redirected.
  const isLoginRequest = path === '/api/v1/auth/login'

  if (res.status === 401 && !isLoginRequest) {
    const { useSessionStore } = await import('../stores/session')
    const session = useSessionStore()
    session.clearSession()

    const currentPath = window.location.pathname + window.location.search
    if (currentPath !== '/login') {
      const { default: router } = await import('../router')
      await router.push({ name: 'login', query: { next: currentPath } })
    }

    return undefined as T
  }

  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }

  if (!res.ok) {
    let message = `HTTP ${res.status}`
    let code: string | undefined
    try {
      const body = (await res.json()) as { error?: string; code?: string }
      if (body.error) message = body.error
      if (body.code) code = body.code
    } catch {
    }
    throw new ApiError(message, res.status, code)
  }

  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}
