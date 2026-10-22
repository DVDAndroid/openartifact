<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- The headline of a version: which key signed it and what that means. Always shown first. -->
<template>
  <section class="rounded-xl border border-border bg-surface p-4" :aria-label="t('android.sig.title')">
    <div class="flex flex-wrap items-center gap-2">
      <h3 class="text-sm font-semibold text-foreground">{{ t('android.sig.title') }}</h3>
      <SignerBadge v-if="version.signer_sha256" :sha256="version.signer_sha256" :trust="signer?.trust" :known="!!signer?.known" :copyable="false" />
      <StatusBadge v-else :label="t('android.upload.unsigned')" variant="danger" />
      <span v-if="signer?.label" class="text-xs text-muted">{{ signer.label }}</span>
    </div>

    <div v-if="version.signer_sha256" class="mt-3">
      <CopyField :value="colonFingerprint(version.signer_sha256)" label="SHA-256" />
    </div>

    <ul class="mt-3 flex flex-wrap gap-2" :aria-label="t('android.sig.schemes')">
      <li v-for="s in SCHEMES" :key="s" class="inline-flex items-center gap-1 text-xs">
        <component :is="schemeState(s) === 'ok' ? CheckCircleIcon : schemeState(s) === 'bad' ? XCircleIcon : MinusCircleIcon"
          class="w-4 h-4"
          :class="schemeState(s) === 'ok' ? 'text-success' : schemeState(s) === 'bad' ? 'text-danger' : 'text-muted'"
          aria-hidden="true" />
        <span :class="schemeState(s) === 'none' ? 'text-muted' : 'text-foreground'">{{ s }}</span>
        <span class="sr-only">{{ schemeState(s) === 'ok' ? t('android.sig.verified') : schemeState(s) === 'bad' ? t('android.sig.failed') : t('android.sig.absent') }}</span>
      </li>
      <li class="inline-flex items-center gap-1 text-xs text-muted" :title="t('android.sig.v4Hint')">
        <MinusCircleIcon class="w-4 h-4" aria-hidden="true" /> v4 · {{ t('android.sig.notApplicable') }}
      </li>
    </ul>

    <ul v-if="notes.length" class="mt-3 space-y-1" role="list">
      <li v-for="n in notes" :key="n.text" class="flex items-start gap-1.5 text-xs" :class="n.danger ? 'text-danger' : 'text-warning'">
        <ExclamationTriangleIcon class="w-4 h-4 shrink-0" aria-hidden="true" />
        {{ n.text }}
      </li>
    </ul>

    <div v-if="parent" class="mt-3 rounded-lg border border-border bg-bg p-3 text-xs space-y-1">
      <p class="font-medium text-foreground">
        {{ t('android.sig.vsOriginal') }}
        <router-link :to="{ query: { ...route.query, v: parent.id } }" class="text-primary hover:underline">
          {{ parent.version_name }} ({{ parent.version_code }})
        </router-link>
      </p>
      <p :class="parent.same_signer ? 'text-success' : 'text-warning'">
        {{ parent.same_signer ? t('android.sig.sameSigner') : t('android.sig.otherSigner') }}
      </p>
      <p v-if="!parent.same_signer" class="text-muted">{{ t('android.sig.reinstallHint') }}</p>
      <p class="text-muted">{{ parent.same_content ? t('android.sig.sameContent') : t('android.sig.modifiedContent') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { CheckCircleIcon, ExclamationTriangleIcon, MinusCircleIcon, XCircleIcon } from '@heroicons/vue/24/outline'
import StatusBadge from '../../../components/StatusBadge.vue'
import { t, tDynamic } from '../../../i18n/en'
import { colonFingerprint } from '../lib'
import type { AndroidVersion, KeyView, ParentView } from '../types'
import CopyField from './CopyField.vue'
import SignerBadge from './SignerBadge.vue'

const props = defineProps<{ version: AndroidVersion; signer: KeyView | null; parent: ParentView | null }>()

const route = useRoute()
const SCHEMES = ['v1', 'v2', 'v3', 'v3.1'] as const

function schemeState(s: string): 'ok' | 'bad' | 'none' {
  const r = props.version.signature?.results.find((x) => x.scheme === s)
  if (!r) return props.version.sig_schemes?.includes(s) ? (props.version.sig_verified ? 'ok' : 'bad') : 'none'
  return r.verified ? 'ok' : 'bad'
}

const notes = computed(() => {
  const out: Array<{ text: string; danger: boolean }> = []
  const sig = props.version.signature
  for (const f of sig?.flags ?? []) out.push({ text: tDynamic(`android.flag.${f}`), danger: f === 'debug_certificate' })
  for (const e of sig?.errors ?? []) out.push({ text: e, danger: true })
  const origin = props.version.origin
  if ((origin === 'built' || origin === 'patched') && props.signer && props.signer.trust !== 'mine') {
    out.push({ text: t('android.sig.notMine'), danger: false })
  }
  if (origin === 'patched' && !props.version.parent_id) out.push({ text: t('android.sig.noOriginal'), danger: false })
  return out
})
</script>
