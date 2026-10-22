<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-4">
    <section v-if="info" class="rounded-xl border border-border bg-surface p-4">
      <h3 class="text-sm font-semibold text-foreground mb-2">{{ t('android.deps.build') }}</h3>
      <dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1.5 text-xs">
        <dt class="text-muted">Android Gradle Plugin</dt><dd class="font-mono text-foreground">{{ info.agp_version || '—' }}</dd>
        <dt class="text-muted">Kotlin</dt><dd class="text-foreground">{{ info.kotlin ? t('android.yes') : t('android.no') }}</dd>
        <dt class="text-muted">Jetpack Compose</dt><dd class="text-foreground">{{ info.compose ? t('android.yes') : t('android.no') }}</dd>
        <dt class="text-muted">{{ t('android.deps.dex') }}</dt><dd class="font-mono text-foreground">{{ info.dex_files }}</dd>
      </dl>
    </section>

    <section class="rounded-xl border border-border bg-surface">
      <header class="flex flex-wrap items-center gap-2 px-4 py-2.5 border-b border-border">
        <h3 class="text-sm font-semibold text-foreground mr-auto">{{ t('android.deps.title') }} <span class="text-muted font-normal">({{ deps.length }})</span></h3>
        <input v-if="deps.length" v-model="filter" type="search" class="h-8 w-48 rounded-lg border border-border-strong bg-surface px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary" :placeholder="t('common.search')" :aria-label="t('common.search')" />
      </header>
      <p v-if="deps.length === 0" class="px-4 py-3 text-sm text-muted">{{ t('android.deps.none') }}</p>
      <table v-else class="w-full text-xs">
        <thead>
          <tr class="border-b border-border bg-surface-hover text-muted font-semibold">
            <th class="text-left px-4 py-2">{{ t('android.deps.coordinate') }}</th>
            <th class="text-left px-4 py-2">{{ t('android.v.version') }}</th>
            <th class="text-left px-4 py-2">{{ t('android.deps.source') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in visible" :key="`${d.group}:${d.artifact}:${d.version}`" class="border-t border-border first:border-t-0">
            <td class="px-4 py-2 font-mono text-foreground">{{ d.group }}:{{ d.artifact }}</td>
            <td class="px-4 py-2 font-mono text-muted">{{ d.version }}</td>
            <td class="px-4 py-2 font-mono text-muted">{{ d.source }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { t } from '../../../i18n/en'
import type { VersionDetail } from '../types'

const props = defineProps<{ detail: VersionDetail }>()

const filter = ref('')
const info = computed(() => props.detail.version.build_info)
const deps = computed(() => props.detail.version.dependencies ?? [])
const visible = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return q ? deps.value.filter((d) => `${d.group}:${d.artifact}`.toLowerCase().includes(q)) : deps.value
})
</script>
