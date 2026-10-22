// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, expect, it } from 'bun:test'
import {
  adbInstall, colonFingerprint, entryCategory, extractFingerprint, isValidTag, normalizeFingerprint,
  parseTags, permissionGroup, sdkLabel, shortFingerprint, titleFor, trustVariant,
} from '../lib'

const SHA = 'ab'.repeat(32)

describe('fingerprints', () => {
  it('shortens to the 8-hex prefix used in download names', () => {
    expect(shortFingerprint(SHA)).toBe('abababab')
    expect(shortFingerprint('')).toBe('')
  })

  it('formats like keytool', () => {
    expect(colonFingerprint('0a1b2c')).toBe('0A:1B:2C')
  })

  it('accepts colons, spaces and case', () => {
    expect(normalizeFingerprint(colonFingerprint(SHA))).toBe(SHA)
    expect(normalizeFingerprint('abc')).toBeNull()
  })

  it('finds the SHA-256 line in keytool -list -v output', () => {
    const out = [
      'Certificate fingerprints:',
      `\t SHA1: ${colonFingerprint('11'.repeat(20))}`,
      `\t SHA256: ${colonFingerprint(SHA)}`,
    ].join('\n')
    expect(extractFingerprint(out)).toBe(SHA)
    expect(extractFingerprint(SHA)).toBe(SHA)
    expect(extractFingerprint('not a fingerprint')).toBeNull()
  })
})

describe('trustVariant', () => {
  it('flags unknown keys, and marks your own as success', () => {
    expect(trustVariant('mine', true)).toBe('success')
    expect(trustVariant('untrusted', true)).toBe('danger')
    expect(trustVariant('original', true)).toBe('neutral')
    expect(trustVariant(undefined, false)).toBe('warning')
  })
})

describe('titleFor', () => {
  const labels = { '': 'App', it: 'Applicazione', 'pt-BR': 'Aplicativo' }

  it('prefers the exact locale, then the language, then the default', () => {
    expect(titleFor(labels, 'x', 'pt-BR')).toBe('Aplicativo')
    expect(titleFor(labels, 'x', 'it-IT')).toBe('Applicazione')
    expect(titleFor(labels, 'x', 'de')).toBe('App')
    expect(titleFor(undefined, 'fallback', 'de')).toBe('fallback')
  })
})

describe('permissionGroup', () => {
  it('splits dangerous, special, normal and custom permissions', () => {
    expect(permissionGroup({ name: 'android.permission.CAMERA' })).toBe('dangerous')
    expect(permissionGroup({ name: 'android.permission.REQUEST_INSTALL_PACKAGES' })).toBe('signature')
    expect(permissionGroup({ name: 'android.permission.INTERNET' })).toBe('normal')
    expect(permissionGroup({ name: 'com.vendor.permission.PUSH' })).toBe('custom')
  })
})

describe('tags', () => {
  it('parses free-form input into unique lowercase tags', () => {
    expect(parseTags('Device:Pixel, patch:revanced  device:pixel')).toEqual(['device:pixel', 'patch:revanced'])
  })

  it('mirrors the server rule', () => {
    expect(isValidTag('base:1.2_x')).toBe(true)
    expect(isValidTag('Has Space')).toBe(false)
    expect(isValidTag('x'.repeat(65))).toBe(false)
  })
})

describe('misc', () => {
  it('names well-known SDK levels', () => {
    expect(sdkLabel(34)).toBe('34 (Android 14)')
    expect(sdkLabel(99)).toBe('99')
    expect(sdkLabel(0)).toBe('—')
  })

  it('categorises zip entries', () => {
    expect(entryCategory('classes2.dex')).toBe('dex')
    expect(entryCategory('res/layout/a.xml')).toBe('resources')
    expect(entryCategory('resources.arsc')).toBe('resources')
    expect(entryCategory('assets/x')).toBe('assets')
    expect(entryCategory('lib/arm64-v8a/liba.so')).toBe('lib')
    expect(entryCategory('META-INF/MANIFEST.MF')).toBe('other')
  })

  it('uses install-multiple for bundles', () => {
    expect(adbInstall('a.apk', 'apk')).toBe('adb install -r a.apk')
    expect(adbInstall('a.apks', 'apks')).toBe('adb install-multiple -r a.apks')
  })
})
