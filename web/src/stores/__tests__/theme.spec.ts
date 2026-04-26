// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect, beforeEach, spyOn } from 'bun:test'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick, ref, type Ref } from 'vue'
import * as vueuse from '@vueuse/core'
import { useThemeStore } from '../theme'

// happy-dom cannot flip prefers-color-scheme, so usePreferredDark() is stubbed
// with a ref the tests drive. useStorage() stays real, so the store's
// serializer and the localStorage round trip are exercised too.
//
// The stub is a spyOn() on the package's export, reverted by mock.restore() in
// the global afterEach (test/isolation.ts). mock.module('@vueuse/core') would
// instead replace the package for every test file that runs afterwards.
let preferredDark: Ref<boolean>

function setPreferredDark(value: boolean): void {
  preferredDark.value = value
}

describe('theme store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    preferredDark = ref(false)
    spyOn(vueuse, 'usePreferredDark').mockReturnValue(preferredDark)
  })

  it('defaults to auto mode', () => {
    const theme = useThemeStore()
    expect(theme.mode).toBe('auto')
  })

  it('persists mode selection', async () => {
    const theme = useThemeStore()
    theme.setMode('dark')
    await nextTick()
    expect(window.localStorage.getItem('oa.theme')).toBe(JSON.stringify('dark'))
  })

  it('resolves auto from prefers-color-scheme', () => {
    setPreferredDark(true)
    const theme = useThemeStore()
    expect(theme.resolved).toBe('dark')
    setPreferredDark(false)
    expect(theme.resolved).toBe('light')
  })

  it('explicit modes override system preference', () => {
    setPreferredDark(true)
    const theme = useThemeStore()
    theme.setMode('light')
    expect(theme.resolved).toBe('light')
    theme.setMode('dark')
    expect(theme.resolved).toBe('dark')
  })

  it('init() applies the .dark class on documentElement', async () => {
    const theme = useThemeStore()
    theme.setMode('dark')
    theme.init()
    await Promise.resolve()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    theme.setMode('light')
    await Promise.resolve()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
