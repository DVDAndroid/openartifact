// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { afterEach, describe, expect, it } from 'bun:test'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useSessionStore, type User } from '../../../stores/session'
import AndroidAppsPanel from '../components/AndroidAppsPanel.vue'
import AndroidSignersPanel from '../components/AndroidSignersPanel.vue'
import AndroidUploadZone from '../components/AndroidUploadZone.vue'
import MarkdownField from '../components/MarkdownField.vue'
import SignatureCard from '../components/SignatureCard.vue'
import SignerBadge from '../components/SignerBadge.vue'
import TagEditor from '../components/TagEditor.vue'
import VersionManifest from '../components/VersionManifest.vue'
import VersionNative from '../components/VersionNative.vue'
import { SIGNER, app, detail, knownKey, mockApi, version } from './helpers'

const realFetch = globalThis.fetch
afterEach(() => {
  globalThis.fetch = realFetch
})

function plugins() {
  const pinia = createPinia()
  setActivePinia(pinia)
  useSessionStore().user = { id: 'u', username: 'me', is_admin: true } as User
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:rest(.*)*', component: { template: '<div />' } }],
  })
  return { plugins: [pinia, router] }
}

describe('SignerBadge', () => {
  it('labels the trust level and shows the short fingerprint', () => {
    const w = mount(SignerBadge, { props: { sha256: SIGNER, trust: 'mine', known: true }, global: plugins() })
    expect(w.text()).toContain('Your key')
    expect(w.text()).toContain('cdcdcdcd')
  })

  it('calls an unregistered key unknown', () => {
    const w = mount(SignerBadge, { props: { sha256: SIGNER, known: false }, global: plugins() })
    expect(w.text()).toContain('Unknown key')
  })
})

describe('TagEditor', () => {
  it('adds a valid tag on Enter and rejects an invalid one', async () => {
    const w = mount(TagEditor, { props: { modelValue: ['a'], editable: true } })
    const input = w.find('input')
    await input.setValue('Patch:ReVanced')
    await input.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('update:modelValue')?.[0]).toEqual([['a', 'patch:revanced']])

    await input.setValue('bad tag')
    await input.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('update:modelValue')).toHaveLength(1)
    expect(w.text()).toContain('Use lowercase letters')
  })

  it('removes a tag', async () => {
    const w = mount(TagEditor, { props: { modelValue: ['a', 'b'], editable: true } })
    await w.find('button[aria-label="Remove tag a"]').trigger('click')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([['b']])
  })

  it('is read-only without edit rights', () => {
    const w = mount(TagEditor, { props: { modelValue: ['a'] } })
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('button').exists()).toBe(false)
  })
})

describe('MarkdownField', () => {
  it('renders notes without interpreting HTML', () => {
    const w = mount(MarkdownField, { props: { title: 'Notes', modelValue: '**hi** <b>x</b>', empty: 'none' } })
    expect(w.html()).toContain('<strong>hi</strong>')
    expect(w.html()).not.toContain('<b>x</b>')
  })

  it('saves the edited text', async () => {
    const w = mount(MarkdownField, { props: { title: 'Notes', modelValue: 'old', empty: 'none', editable: true } })
    await w.find('button').trigger('click')
    await w.find('textarea').setValue('new')
    const save = w.findAll('button').find((b) => b.text() === 'Save')!
    await save.trigger('click')
    expect(w.emitted('save')?.[0]).toEqual(['new'])
  })
})

describe('SignatureCard', () => {
  const mountCard = (over: Parameters<typeof detail>[0], extra: Parameters<typeof detail>[1] = {}) => {
    const d = detail(over, extra)
    return mount(SignatureCard, { props: { version: d.version, signer: d.signer, parent: d.parent }, global: plugins() })
  }

  it('leads with the key, its trust and the verified schemes', () => {
    const w = mountCard({})
    expect(w.text()).toContain('Your key')
    expect(w.text()).toContain('CD:CD:CD')
    expect(w.text()).toContain('v2')
    expect(w.text()).toContain('v3.1')
  })

  it('warns about a debug certificate', () => {
    const d = detail({})
    d.version.signature!.flags = ['debug_certificate']
    const w = mount(SignatureCard, { props: { version: d.version, signer: d.signer, parent: null }, global: plugins() })
    expect(w.text()).toContain('Android debug certificate')
  })

  it('explains a patched file signed with another key than its original', () => {
    const w = mountCard({ origin: 'patched', parent_id: 'p' }, {
      parent: { id: 'p', version_name: '1.0', version_code: 7, origin: 'original', signer_sha256: 'aa'.repeat(32), same_signer: false, same_content: false },
    })
    expect(w.text()).toContain('different key than the original')
    expect(w.text()).toContain('uninstall the original first')
  })

  it('warns when an own build is signed with a key that is not yours', () => {
    const d = detail({ origin: 'built' })
    d.signer = { sha256: SIGNER, known: false }
    const w = mount(SignatureCard, { props: { version: d.version, signer: d.signer, parent: null }, global: plugins() })
    expect(w.text()).toContain('not signed with a key marked as yours')
  })
})

describe('VersionManifest', () => {
  it('groups permissions by risk and filters them', async () => {
    const d = detail({
      manifest: {
        package: 'org.example.app', version_code: 7, version_name: '1.2.3', min_sdk: 24, target_sdk: 34,
        application: { debuggable: false }, features: [], components: [],
        permissions: [{ name: 'android.permission.CAMERA' }, { name: 'android.permission.INTERNET' }, { name: 'com.x.PUSH' }],
      },
    })
    const w = mount(VersionManifest, { props: { repo: 'r', detail: d }, global: plugins() })
    expect(w.text()).toContain('Dangerous (asked at runtime) (1)')
    expect(w.text()).toContain('CAMERA')
    await w.find('input[type="search"]').setValue('inter')
    expect(w.text()).toContain('INTERNET')
    expect(w.text()).not.toContain('CAMERA')
  })
})

describe('VersionNative', () => {
  it('lists libraries per ABI and the 16 KiB verdict', () => {
    const d = detail({
      abis: ['arm64-v8a'], page_size_16k: false, frameworks: ['flutter'],
      native_libs: [{ abi: 'arm64-v8a', name: 'libapp.so', path: 'lib/arm64-v8a/libapp.so', size: 10, stored: true, aligned_16k: false, stripped: true, soname: 'libapp.so' }],
    })
    const w = mount(VersionNative, { props: { detail: d } })
    expect(w.text()).toContain('arm64-v8a (1)')
    expect(w.text()).toContain('libapp.so')
    expect(w.text()).toContain('not aligned for 16 KiB pages')
    expect(w.text()).toContain('flutter')
  })
})

describe('AndroidAppsPanel', () => {
  const routes = (apps = [app()]) => ({
    'GET /api/v1/repositories/phone/apks/apps': () => ({ items: apps, total: apps.length, page: 1, limit: 25 }),
    'GET /api/v1/repositories/phone/apks/tags': () => [{ tag: 'device:pixel', apps: 1, versions: 2 }],
    'GET /api/v1/repositories/phone/apks/signers': () => [knownKey()],
  })

  it('lists apps with their latest version, origins and signer trust', async () => {
    mockApi(routes())
    const w = mount(AndroidAppsPanel, { props: { repo: 'phone', canWrite: true }, global: plugins() })
    await flushPromises()
    expect(w.text()).toContain('Example')
    expect(w.text()).toContain('org.example.app')
    expect(w.text()).toContain('1.2.3')
    expect(w.text()).toContain('Built')
    expect(w.text()).toContain('Original')
    expect(w.text()).toContain('Your key')
    expect(w.find('a[href="/repos/phone/packages/android/org.example.app"]').exists()).toBe(true)
    expect(w.emitted('loaded')?.[0]).toEqual([1])
  })

  it('turns a tag chip into a filter on the request', async () => {
    const { calls } = mockApi(routes())
    const w = mount(AndroidAppsPanel, { props: { repo: 'phone', canWrite: false }, global: plugins() })
    await flushPromises()
    await w.find('button[aria-pressed]').trigger('click')
    await new Promise((r) => setTimeout(r, 300))
    await flushPromises()
    const last = calls.filter((c) => c.url.includes('/apps?')).at(-1)!
    expect(new URL(last.url, 'http://x').searchParams.get('tag')).toBe('device:pixel')
  })

  it('shows the empty state with the F-Droid hint', async () => {
    mockApi(routes([]))
    const w = mount(AndroidAppsPanel, { props: { repo: 'phone', canWrite: true }, global: plugins() })
    await flushPromises()
    expect(w.text()).toContain('No apps yet')
    expect(w.text()).toContain('f-droid.org/repo')
  })

  it('hides upload without write access', async () => {
    mockApi(routes())
    const w = mount(AndroidAppsPanel, { props: { repo: 'phone', canWrite: false }, global: plugins() })
    await flushPromises()
    expect(w.text()).not.toContain('Upload APK')
  })
})

describe('AndroidUploadZone', () => {
  const inspected = {
    info: {
      manifest: { package: 'org.example.app', version_code: 8, version_name: '1.3.0', min_sdk: 24, target_sdk: 34 },
      label: 'Example', labels: {}, icons: [], content_digest: 'x', abis: ['arm64-v8a'], native_libs: [], page_size_16k: true,
      frameworks: [], warnings: ['unaligned entries'],
      signature: { schemes: ['v2'], verified: true, results: [], signers: [], signer: { sha256: SIGNER } },
    },
    sha256: 'ab', size_bytes: 3, filename: 'a.apk', duplicate: null,
    previous: version({ signer_sha256: 'aa'.repeat(32) }),
  }

  async function pick(w: ReturnType<typeof mount>): Promise<void> {
    const input = w.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [new File(['abc'], 'a.apk')], configurable: true })
    await input.trigger('change')
    await flushPromises()
  }

  it('previews the signer and compares it with the latest stored version', async () => {
    mockApi({ 'POST /api/v1/repositories/phone/apks/inspect': () => inspected })
    const w = mount(AndroidUploadZone, { props: { repo: 'phone' }, global: plugins() })
    await pick(w)
    expect(w.text()).toContain('org.example.app')
    expect(w.text()).toContain('1.3.0 (8)')
    expect(w.text()).toContain('CD:CD:CD')
    expect(w.text()).toContain('unaligned entries')
    expect(w.text()).toContain('Different signing key')
  })

  it('asks for the original of a patched file and warns without it', async () => {
    mockApi({ 'POST /api/v1/repositories/phone/apks/inspect': () => inspected })
    const w = mount(AndroidUploadZone, { props: { repo: 'phone' }, global: plugins() })
    await pick(w)
    expect(w.text()).not.toContain('Original file')
    await w.find('select').setValue('patched')
    expect(w.text()).toContain('Original file')
    expect(w.text()).toContain('cannot be compared with what it came from')
  })

  it('reports a file that cannot be analysed', async () => {
    mockApi({
      'POST /api/v1/repositories/phone/apks/inspect': () =>
        new Response(JSON.stringify({ error: 'not a valid APK', code: 'INVALID_INPUT' }), { status: 400 }),
    })
    const w = mount(AndroidUploadZone, { props: { repo: 'phone' }, global: plugins() })
    await pick(w)
    expect(w.text()).toContain('not a valid APK')
  })
})

describe('AndroidSignersPanel', () => {
  it('lists known keys', async () => {
    mockApi({ 'GET /api/v1/repositories/phone/apks/signers': () => [knownKey()] })
    const w = mount(AndroidSignersPanel, { props: { repo: 'phone', canWrite: true }, global: plugins() })
    await flushPromises()
    expect(w.text()).toContain('release key')
    expect(w.text()).toContain('Your key')
    expect(w.text()).toContain('Whole repository')
  })

  it('registers a key pasted as keytool output', async () => {
    const { calls } = mockApi({
      'GET /api/v1/repositories/phone/apks/signers': () => [],
      'POST /api/v1/repositories/phone/apks/signers': () => knownKey(),
    })
    const w = mount(AndroidSignersPanel, { props: { repo: 'phone', canWrite: true }, global: plugins() })
    await flushPromises()
    const colon = (SIGNER.toUpperCase().match(/.{2}/g) ?? []).join(':')
    await w.find('textarea').setValue(`Certificate fingerprints:\n\t SHA256: ${colon}`)
    await w.find('form').trigger('submit')
    await flushPromises()
    const post = calls.find((c) => c.init.method === 'POST')!
    expect(JSON.parse(String(post.init.body))).toMatchObject({ sha256: SIGNER, trust: 'mine' })
  })

  it('refuses a private key', async () => {
    mockApi({ 'GET /api/v1/repositories/phone/apks/signers': () => [] })
    const w = mount(AndroidSignersPanel, { props: { repo: 'phone', canWrite: true }, global: plugins() })
    await flushPromises()
    await w.find('textarea').setValue('-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----')
    expect(w.text()).toContain('looks like a private key')
    expect(w.find('button[type="submit"]').attributes('disabled')).toBeDefined()
  })
})

describe('empty lists from the server', () => {
  it('renders a manifest whose lists are null (Go encodes empty slices as null)', () => {
    const d = detail({
      manifest: {
        package: 'p', version_code: 1, version_name: '1', min_sdk: 21, target_sdk: 34, application: { debuggable: false },
        permissions: null, features: null, components: null,
      } as never,
    })
    const w = mount(VersionManifest, { props: { repo: 'r', detail: d }, global: plugins() })
    expect(w.text()).toContain('This app requests no permissions.')
  })
})
