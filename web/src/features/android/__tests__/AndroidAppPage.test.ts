// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { afterEach, describe, expect, it } from 'bun:test'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useSessionStore, type User } from '../../../stores/session'
import { androidRoutes } from '../routes'
import AndroidAppPage from '../pages/AndroidAppPage.vue'
import { SIGNER, detail, knownKey, mockApi, version } from './helpers'

const realFetch = globalThis.fetch
afterEach(() => {
  globalThis.fetch = realFetch
})

const OTHER = 'aa'.repeat(32)
const V1 = '11111111-1111-1111-1111-111111111111'
const V2 = '22222222-2222-2222-2222-222222222222'

function routes(extra: Record<string, () => unknown> = {}) {
  const orig = version({ id: V2, origin: 'original', version_name: '1.2.3', signer_sha256: OTHER, label: 'Example' })
  const mine = version({ id: V1, origin: 'patched', parent_id: V2 })
  return {
    // The repository endpoint is not part of the android API: no {success, data} envelope.
    'GET /api/v1/repository/phone': () =>
      new Response(JSON.stringify({ id: 'r', name: 'phone', format: 'android', repo_type: 'hosted', permissions: { write: true, delete: true, manage: false } })),
    'GET /api/v1/repositories/phone/apks/apps/org.example.app': () => ({
      app: { id: 'a', repository_id: 'r', package_name: 'org.example.app', notes: 'my *notes*', tags: ['t1'], signer_policy: 'warn', created_at: '', updated_at: '' },
      signers: [
        { sha256: SIGNER, versions: 1, first: '2025-01-01T00:00:00Z', last: '2025-01-02T00:00:00Z', origins: ['patched'], known: false },
        { sha256: OTHER, versions: 1, first: '2025-01-01T00:00:00Z', last: '2025-01-01T00:00:00Z', origins: ['original'], known: true, trust: 'original' },
      ],
    }),
    'GET /api/v1/repositories/phone/apks/apps/org.example.app/versions': () => ({ items: [mine, orig], total: 2, page: 1, limit: 200 }),
    'GET /api/v1/repositories/phone/apks/signers': () => [knownKey({ sha256: OTHER, trust: 'original', label: 'publisher' })],
    'GET /api/v1/repositories/phone/apks/tags': () => [{ tag: 't1', apps: 1, versions: 1 }],
    [`GET /api/v1/repositories/phone/apks/versions/${V1}`]: () =>
      detail({ id: V1, origin: 'patched', parent_id: V2 }, {
        signer: { sha256: SIGNER, known: false },
        parent: { id: V2, version_name: '1.2.3', version_code: 7, origin: 'original', signer_sha256: OTHER, same_signer: false, same_content: false },
      }),
    ...extra,
  }
}

async function mountPage(path = '/repos/phone/packages/android/org.example.app', extra: Record<string, () => unknown> = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().user = { id: 'u', username: 'me', is_admin: false } as User
  const api = mockApi(routes(extra))
  const router = createRouter({ history: createMemoryHistory(), routes: [...androidRoutes, { path: '/repos', component: { template: '<div />' } }, { path: '/repos/:name', component: { template: '<div />' } }] })
  await router.push(path)
  const w = mount(AndroidAppPage, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { w, router, api }
}

describe('AndroidAppPage', () => {
  it('shows the keys used by the app and flags an own build signed with a foreign key', async () => {
    const { w } = await mountPage()
    expect(w.text()).toContain('Signing keys')
    expect(w.text()).toContain('Original publisher')
    expect(w.text()).toContain('Unknown key')
    expect(w.text()).toContain('Your own build is signed with a key not marked as yours.')
  })

  it('lists the versions and opens the newest with its signature card', async () => {
    const { w } = await mountPage()
    expect(w.findAll('tbody tr').length).toBeGreaterThanOrEqual(2)
    expect(w.text()).toContain('Signature')
    expect(w.text()).toContain('different key than the original')
    expect(w.text()).toContain('Patched')
  })

  it('renders the notes as markdown', async () => {
    const { w } = await mountPage()
    expect(w.html()).toContain('<em>notes</em>')
  })

  it('selects the version named in the query string', async () => {
    const orig = detail({ id: V2, origin: 'original', version_name: 'ORIGINAL-BUILD', signer_sha256: OTHER })
    const { w } = await mountPage(`/repos/phone/packages/android/org.example.app?v=${V2}`, {
      [`GET /api/v1/repositories/phone/apks/versions/${V2}`]: () => orig,
    })
    expect(w.text()).toContain('ORIGINAL-BUILD')
  })

  it('switches tab through the query string', async () => {
    const { w, router } = await mountPage()
    await router.push({ query: { tab: 'native' } })
    await flushPromises()
    expect(w.text()).toContain('This file has no native libraries.')
  })

  it('deletes a version after confirmation', async () => {
    const { w, api } = await mountPage('/repos/phone/packages/android/org.example.app', {
      [`DELETE /api/v1/repositories/phone/apks/versions/${V2}`]: () => ({}),
    })
    // Rows come newest first: the second one is the original (V2).
    await w.findAll('button[aria-label="Delete 1.2.3"]')[1].trigger('click')
    await flushPromises()
    const confirm = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Delete' && b.closest('[role="dialog"]'))
    expect(confirm).toBeTruthy()
    confirm!.click()
    await flushPromises()
    expect(api.calls.some((c) => c.init.method === 'DELETE' && c.url.endsWith(`/versions/${V2}`))).toBe(true)
  })

  it('says so when the app does not exist', async () => {
    const { w } = await mountPage('/repos/phone/packages/android/org.example.app', {
      'GET /api/v1/repositories/phone/apks/apps/org.example.app': () =>
        new Response(JSON.stringify({ error: 'app not found', code: 'NOT_FOUND' }), { status: 404 }),
    })
    expect(w.text()).toContain('App not found')
  })
})
