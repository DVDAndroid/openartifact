<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- A value in monospace with a copy button, for URLs, hashes and commands. -->
<template>
  <div class="flex items-center gap-2 min-w-0">
    <span v-if="label" class="text-xs text-muted shrink-0">{{ label }}</span>
    <code class="flex-1 min-w-0 rounded-md border border-border bg-surface-hover px-2.5 py-1 text-xs font-mono text-foreground break-all">{{ value }}</code>
    <AppButton variant="secondary" size="xs" class="shrink-0" :aria-label="`${t('repoDetail.copy')} ${label ?? ''}`.trim()" @click="copy">
      <ClipboardIcon class="w-3.5 h-3.5 text-muted" aria-hidden="true" />
      {{ t('repoDetail.copy') }}
    </AppButton>
  </div>
</template>

<script setup lang="ts">
import { ClipboardIcon } from '@heroicons/vue/24/outline'
import AppButton from '../../../components/AppButton.vue'
import { useToastStore } from '../../../stores/toast'
import { t } from '../../../i18n/en'

const props = defineProps<{ value: string; label?: string }>()
const toasts = useToastStore()

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.value)
    toasts.success(t('android.copied'))
  } catch {
    toasts.error(t('android.copyFailed'))
  }
}
</script>
