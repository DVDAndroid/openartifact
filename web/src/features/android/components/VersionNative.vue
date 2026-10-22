<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-4">
    <section class="rounded-xl border border-border bg-surface p-4 flex flex-wrap gap-x-8 gap-y-2 text-xs">
      <p><span class="text-muted">{{ t('android.v.frameworks') }}:</span> <span class="text-foreground">{{ v.frameworks?.length ? v.frameworks.join(', ') : '—' }}</span></p>
      <p><span class="text-muted">{{ t('android.v.abis') }}:</span> <span class="font-mono text-foreground">{{ v.abis.length ? v.abis.join(', ') : '—' }}</span></p>
      <p v-if="v.abis.length">
        <span class="text-muted">16 KiB:</span>
        <span :class="v.page_size_16k ? 'text-success' : 'text-warning'"> {{ v.page_size_16k ? t('android.native.ok16k') : t('android.native.no16k') }}</span>
      </p>
    </section>

    <p v-if="libs.length === 0" class="text-sm text-muted">{{ t('android.native.none') }}</p>

    <section v-for="g in byAbi" :key="g.abi" class="rounded-xl border border-border bg-surface">
      <header class="px-4 py-2.5 border-b border-border">
        <h3 class="text-sm font-semibold font-mono text-foreground">{{ g.abi }} <span class="text-muted font-normal">({{ g.libs.length }})</span></h3>
      </header>
      <div class="overflow-x-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="border-b border-border bg-surface-hover text-muted font-semibold">
              <th class="text-left px-4 py-2">{{ t('android.native.file') }}</th>
              <th class="text-right px-4 py-2">{{ t('android.col.size') }}</th>
              <th class="text-left px-4 py-2">SONAME</th>
              <th class="text-left px-4 py-2">{{ t('android.native.needed') }}</th>
              <th class="text-left px-4 py-2">Build ID</th>
              <th class="text-left px-4 py-2">{{ t('android.native.stripped') }}</th>
              <th class="text-left px-4 py-2">16 KiB</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in g.libs" :key="l.path" class="border-t border-border first:border-t-0 align-top">
              <td class="px-4 py-2 font-mono text-foreground">{{ l.name }}<span v-if="l.error" class="block text-danger">{{ l.error }}</span></td>
              <td class="px-4 py-2 text-right font-mono tabular-nums text-muted">{{ formatBytes(l.size) }}</td>
              <td class="px-4 py-2 font-mono text-muted">{{ l.soname || '—' }}</td>
              <td class="px-4 py-2 font-mono text-muted">{{ l.needed?.join(', ') || '—' }}</td>
              <td class="px-4 py-2 font-mono text-muted break-all" :title="l.build_id">{{ l.build_id ? l.build_id.slice(0, 16) : '—' }}</td>
              <td class="px-4 py-2 text-foreground">{{ l.stripped ? t('android.yes') : t('android.no') }}</td>
              <td class="px-4 py-2" :class="l.aligned_16k ? 'text-success' : 'text-warning'">{{ l.aligned_16k ? t('android.yes') : t('android.no') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { t } from '../../../i18n/en'
import { formatBytes } from '../lib'
import type { VersionDetail } from '../types'

const props = defineProps<{ detail: VersionDetail }>()

const v = computed(() => props.detail.version)
const libs = computed(() => v.value.native_libs ?? [])
const byAbi = computed(() => {
  const by = new Map<string, typeof libs.value>()
  for (const l of libs.value) by.set(l.abi, [...(by.get(l.abi) ?? []), l])
  return [...by.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([abi, list]) => ({ abi, libs: list }))
})
</script>
