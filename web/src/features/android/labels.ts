// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { tDynamic } from '../../i18n/en'
import type { Origin, Trust } from './types'

export function originLabel(o: Origin | string): string {
  return tDynamic(`android.origin.${o}`)
}

export function trustLabel(trust: Trust | string): string {
  return tDynamic(`android.trust.${trust}`)
}

export function policyLabel(policy: string): string {
  return tDynamic(`android.policy.${policy}`)
}

export function fileKindLabel(kind: string): string {
  return tDynamic(`android.fileKind.${kind}`)
}
