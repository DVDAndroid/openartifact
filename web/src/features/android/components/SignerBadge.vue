<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- Who signed a file: trust label, short SHA-256 fingerprint and a copy button for the full value. -->
<template>
  <span class="inline-flex items-center gap-1.5 min-w-0">
    <StatusBadge :label="badgeLabel" :variant="variant" />
    <code class="font-mono text-xs text-muted" :title="colonFingerprint(sha256)">{{ shortFingerprint(sha256) }}</code>
    <button
      v-if="copyable"
      type="button"
      class="shrink-0 rounded p-0.5 text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
      :title="t('android.signer.copy')"
      :aria-label="t('android.signer.copy')"
      @click.stop="copy"
    >
      <ClipboardIcon class="w-3.5 h-3.5" aria-hidden="true" />
    </button>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ClipboardIcon } from '@heroicons/vue/24/outline'
import StatusBadge from '../../../components/StatusBadge.vue'
import { useToastStore } from '../../../stores/toast'
import { t } from '../../../i18n/en'
import { colonFingerprint, shortFingerprint, trustVariant } from '../lib'
import type { Trust } from '../types'

const props = withDefaults(
  defineProps<{ sha256: string; trust?: Trust; known?: boolean; label?: string; copyable?: boolean }>(),
  { trust: undefined, known: false, label: undefined, copyable: true },
)

const toasts = useToastStore()

const variant = computed(() => trustVariant(props.trust, props.known))

const badgeLabel = computed(() => {
  if (!props.known || !props.trust) return t('android.trust.unknown')
  switch (props.trust) {
    case 'mine':
      return t('android.trust.mine')
    case 'original':
      return t('android.trust.original')
    case 'third_party':
      return t('android.trust.third_party')
    default:
      return t('android.trust.untrusted')
  }
})

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.sha256)
    toasts.success(t('android.signer.copied'))
  } catch {
    toasts.error(t('android.signer.copyFailed'))
  }
}
</script>
