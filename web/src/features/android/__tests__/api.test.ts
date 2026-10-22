// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { afterEach, describe, expect, it } from 'bun:test'
import { createSigner, inspectApk, listApps, updateApp } from '../api'
import { app, mockApi } from './helpers'

const realFetch = globalThis.fetch
afterEach(() => {
  globalThis.fetch = realFetch
})

describe('android api', () => {
  it('unwraps the envelope and encodes the filter', async () => {
    const { calls } = mockApi({
      'GET /api/v1/repositories/my%20repo/apks/apps': () => ({ items: [app()], total: 1, page: 1, limit: 25 }),
    })
    const page = await listApps('my repo', { q: 'cam', tags: ['a', 'b'], origin: 'patched', sort: 'size', order: 'desc', page: 2, limit: 25 })
    expect(page.total).toBe(1)
    expect(page.items[0].package_name).toBe('org.example.app')

    const qs = new URL(calls[0].url, 'http://x').searchParams
    expect(qs.get('q')).toBe('cam')
    expect(qs.get('tag')).toBe('a,b')
    expect(qs.get('origin')).toBe('patched')
    expect(qs.get('sort')).toBe('size')
    expect(qs.get('page')).toBe('2')
  })

  it('leaves empty filters out of the URL', async () => {
    const { calls } = mockApi({
      'GET /api/v1/repositories/r/apks/apps': () => ({ items: [], total: 0, page: 1, limit: 50 }),
    })
    await listApps('r', { q: '', tags: [], origin: '' })
    expect(calls[0].url).toBe('/api/v1/repositories/r/apks/apps')
  })

  it('sends metadata updates as PUT with the CSRF header', async () => {
    document.cookie = 'oa_csrf=tok'
    const { calls } = mockApi({
      'PUT /api/v1/repositories/r/apks/apps/org.example.app/meta': () => app(),
    })
    await updateApp('r', 'org.example.app', { tags: ['x'] })
    const init = calls[0].init
    expect(init.method).toBe('PUT')
    expect(JSON.parse(String(init.body))).toEqual({ tags: ['x'] })
    expect(new Headers(init.headers).get('X-CSRF-Token')).toBe('tok')
  })

  it('posts the file to inspect as multipart', async () => {
    const { calls } = mockApi({ 'POST /api/v1/repositories/r/apks/inspect': () => ({ sha256: 'a' }) })
    await inspectApk('r', new File(['x'], 'a.apk'))
    expect(calls[0].init.body).toBeInstanceOf(FormData)
    expect((calls[0].init.body as FormData).get('file')).toBeInstanceOf(File)
  })

  it('registers a key by fingerprint', async () => {
    const { calls } = mockApi({ 'POST /api/v1/repositories/r/apks/signers': () => ({ id: 's' }) })
    await createSigner('r', { sha256: 'ab'.repeat(32), trust: 'mine' })
    expect(JSON.parse(String(calls[0].init.body))).toEqual({ sha256: 'ab'.repeat(32), trust: 'mine' })
  })
})
