<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <footer
    class="border-t border-border bg-surface text-xs text-muted flex items-center gap-4 px-4 py-1.5"
    role="contentinfo"
  >
    <span class="flex items-center gap-1.5">
      <span
        class="inline-block w-2 h-2 rounded-full"
        :class="dbHealthy ? 'bg-success' : 'bg-danger'"
        aria-hidden="true"
      />
      {{ t('status.database') }}: {{ sysInfo?.db_status ?? t('status.unknown') }}
    </span>
    <span v-if="sysInfo?.version" class="flex items-center gap-1.5">
      {{ t('status.version') }}: <span class="text-foreground">{{ sysInfo.version }}</span>
    </span>
    <span v-if="sysInfo?.uptime" class="flex items-center gap-1.5">
      {{ sysInfo.uptime }}
    </span>
    <span class="ml-auto" :class="overall === 'ok' ? 'text-success' : 'text-danger'">
      {{ overall === 'ok' ? t('status.healthy') : t('status.degraded') }}
    </span>
  </footer>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { getSystemInfo } from '../api/admin'
import type { SystemInfo as GeneratedSystemInfo } from '../api/generated/types.gen'
import { t } from '../i18n/en'

// The generated SystemInfo may lack some runtime fields; augment as needed
type SystemInfo = GeneratedSystemInfo & {
  uptime?: string
  db_status?: string
}

const sysInfo = ref<SystemInfo | null>(null)
let timer: ReturnType<typeof setInterval> | null = null

const dbHealthy = computed(() => sysInfo.value?.db_status === 'ok')
const overall = computed<'ok' | 'degraded'>(() => (dbHealthy.value ? 'ok' : 'degraded'))

async function refresh(): Promise<void> {
  try {
    sysInfo.value = await getSystemInfo()
  } catch {
    sysInfo.value = null
  }
}

onMounted(() => {
  void refresh()
  timer = setInterval(refresh, 30_000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>
