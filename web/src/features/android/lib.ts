// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { Origin, Permission, Trust } from './types'

/** First 8 hex digits of a fingerprint (the same prefix the download file names carry). */
export function shortFingerprint(sha: string): string {
  return sha ? sha.slice(0, 8) : ''
}

/** AB:CD:… as keytool and apksigner print a SHA-256 fingerprint. */
export function colonFingerprint(sha: string): string {
  return (sha.toUpperCase().match(/.{1,2}/g) ?? []).join(':')
}

export function formatBytes(bytes?: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let i = 0
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return i === 0 ? `${size} B` : `${size.toFixed(1)} ${units[i]}`
}

export type BadgeVariant = 'success' | 'warning' | 'neutral' | 'danger'

export function trustVariant(trust?: Trust, known = true): BadgeVariant {
  if (!known) return 'warning'
  switch (trust) {
    case 'mine':
      return 'success'
    case 'untrusted':
      return 'danger'
    default:
      return 'neutral'
  }
}

export function originVariant(origin: Origin): BadgeVariant {
  switch (origin) {
    case 'built':
      return 'success'
    case 'patched':
      return 'warning'
    default:
      return 'neutral'
  }
}

/** Android API level → release name, for the levels apps target today. */
const SDK_NAMES: Record<number, string> = {
  21: '5.0', 22: '5.1', 23: '6.0', 24: '7.0', 25: '7.1', 26: '8.0', 27: '8.1', 28: '9', 29: '10',
  30: '11', 31: '12', 32: '12L', 33: '13', 34: '14', 35: '15', 36: '16',
}

export function sdkLabel(level?: number): string {
  if (!level) return '—'
  const name = SDK_NAMES[level]
  return name ? `${level} (Android ${name})` : String(level)
}

/** Title for a locale code ("" is the default), falling back to the default title. */
export function titleFor(labels: Record<string, string> | undefined, fallback: string, locale: string): string {
  if (!labels) return fallback
  const lang = locale.toLowerCase().split('-')[0]
  return labels[locale] ?? labels[lang] ?? labels[''] ?? fallback
}

export type PermissionGroup = 'dangerous' | 'signature' | 'normal' | 'custom'

// Permissions that need a runtime grant. Not exhaustive; anything else under
// android.permission.* counts as normal, anything outside it as custom.
const DANGEROUS = new Set([
  'READ_CALENDAR', 'WRITE_CALENDAR', 'CAMERA', 'READ_CONTACTS', 'WRITE_CONTACTS', 'GET_ACCOUNTS',
  'ACCESS_FINE_LOCATION', 'ACCESS_COARSE_LOCATION', 'ACCESS_BACKGROUND_LOCATION', 'RECORD_AUDIO',
  'READ_PHONE_STATE', 'READ_PHONE_NUMBERS', 'CALL_PHONE', 'ANSWER_PHONE_CALLS', 'READ_CALL_LOG',
  'WRITE_CALL_LOG', 'ADD_VOICEMAIL', 'USE_SIP', 'BODY_SENSORS', 'ACTIVITY_RECOGNITION', 'SEND_SMS',
  'RECEIVE_SMS', 'READ_SMS', 'RECEIVE_WAP_PUSH', 'RECEIVE_MMS', 'READ_EXTERNAL_STORAGE',
  'WRITE_EXTERNAL_STORAGE', 'ACCESS_MEDIA_LOCATION', 'READ_MEDIA_IMAGES', 'READ_MEDIA_VIDEO',
  'READ_MEDIA_AUDIO', 'POST_NOTIFICATIONS', 'NEARBY_WIFI_DEVICES', 'BLUETOOTH_CONNECT',
  'BLUETOOTH_SCAN', 'BLUETOOTH_ADVERTISE', 'UWB_RANGING', 'READ_MEDIA_VISUAL_USER_SELECTED',
])
const SIGNATURE = new Set([
  'INSTALL_PACKAGES', 'DELETE_PACKAGES', 'WRITE_SECURE_SETTINGS', 'BIND_ACCESSIBILITY_SERVICE',
  'BIND_DEVICE_ADMIN', 'MANAGE_EXTERNAL_STORAGE', 'SYSTEM_ALERT_WINDOW', 'WRITE_SETTINGS',
  'REQUEST_INSTALL_PACKAGES', 'PACKAGE_USAGE_STATS', 'QUERY_ALL_PACKAGES', 'READ_LOGS', 'DUMP',
])

export function permissionGroup(p: Permission): PermissionGroup {
  const prefix = 'android.permission.'
  if (!p.name.startsWith(prefix)) return 'custom'
  const short = p.name.slice(prefix.length)
  if (DANGEROUS.has(short)) return 'dangerous'
  if (SIGNATURE.has(short) || p.protection_level?.includes('signature')) return 'signature'
  return 'normal'
}

export const PERMISSION_GROUPS: readonly PermissionGroup[] = ['dangerous', 'signature', 'normal', 'custom']

/** Drops the android.permission. prefix so a long list stays scannable. */
export function shortPermission(name: string): string {
  return name.startsWith('android.permission.') ? name.slice('android.permission.'.length) : name
}

/** Splits a free-form tag input ("a, b  c") into tokens, lowercased. */
export function parseTags(input: string): string[] {
  return [...new Set(input.toLowerCase().split(/[\s,]+/).filter(Boolean))]
}

const TAG_RE = /^[a-z0-9._:-]{1,64}$/

export function isValidTag(tag: string): boolean {
  return TAG_RE.test(tag)
}

/** adb command that installs a downloaded file; bundles need install-multiple. */
export function adbInstall(filename: string, fileKind: string): string {
  return fileKind === 'apk' ? `adb install -r ${filename}` : `adb install-multiple -r ${filename}`
}

/** True when an SHA-256 fingerprint (hex, with or without colons) is complete. */
export function normalizeFingerprint(input: string): string | null {
  const hex = input.replace(/[\s:]/g, '').toLowerCase()
  return /^[0-9a-f]{64}$/.test(hex) ? hex : null
}

/** Reads the SHA-256 line out of `keytool -list -v` output, or a bare fingerprint. */
export function extractFingerprint(text: string): string | null {
  const labelled = /SHA-?256\s*(?:digest|fingerprint)?\s*[:=]\s*([0-9A-Fa-f:\s]{64,})/i.exec(text)
  return normalizeFingerprint(labelled ? labelled[1] : text)
}

export type EntryCategory = 'dex' | 'resources' | 'assets' | 'lib' | 'other'

/** Where a zip entry's bytes are counted in the size breakdown. */
export function entryCategory(name: string): EntryCategory {
  if (/^classes\d*\.dex$/.test(name)) return 'dex'
  if (name === 'resources.arsc' || name.startsWith('res/')) return 'resources'
  if (name.startsWith('assets/')) return 'assets'
  if (name.startsWith('lib/')) return 'lib'
  return 'other'
}
