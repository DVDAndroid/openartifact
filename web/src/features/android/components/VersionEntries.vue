<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- The entries of the zip, and how its size splits between code, resources, assets and libraries. -->
<template>
  <div class="space-y-4">
    <section class="rounded-xl border border-border bg-surface p-4">
      <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('android.files.bySize') }}</h3>
      <div v-if="total > 0" class="flex h-3 w-full overflow-hidden rounded-full bg-surface-hover" role="img" :aria-label="t('android.files.bySize')">
        <span v-for="(c, i) in categories" :key="c.id" class="h-full" :style="{ width: `${(c.size / total) * 100}%`, background: chartColor(i) }" :title="`${tDynamic(`android.files.cat.${c.id}`)}: ${formatBytes(c.size)}`" />
      </div>
      <ul class="mt-3 flex flex-wrap gap-x-5 gap-y-1.5">
        <li v-for="(c, i) in categories" :key="c.id" class="flex items-center gap-1.5 text-xs">
          <span class="h-2.5 w-2.5 rounded-sm" :style="{ background: chartColor(i) }" aria-hidden="true" />
          <span class="text-foreground">{{ tDynamic(`android.files.cat.${c.id}`) }}</span>
          <span class="font-mono text-muted">{{ formatBytes(c.size) }} · {{ c.count }}</span>
        </li>
      </ul>
    </section>

    <section class="rounded-xl border border-border bg-surface">
      <header class="flex flex-wrap items-center gap-2 px-4 py-2.5 border-b border-border">
        <h3 class="text-sm font-semibold text-foreground mr-auto">{{ t('android.files.entries') }} <span class="text-muted font-normal">({{ entries.length }})</span></h3>
        <input v-model="filter" type="search" class="h-8 w-56 rounded-lg border border-border-strong bg-surface px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary" :placeholder="t('common.search')" :aria-label="t('common.search')" />
      </header>
      <div class="max-h-[32rem] overflow-auto">
        <table class="w-full text-xs">
          <thead class="sticky top-0">
            <tr class="border-b border-border bg-surface-hover text-muted font-semibold">
              <th class="text-left px-4 py-2">{{ t('android.native.file') }}</th>
              <th class="text-right px-4 py-2">{{ t('android.col.size') }}</th>
              <th class="text-right px-4 py-2">{{ t('android.files.compressed') }}</th>
              <th class="text-left px-4 py-2">CRC-32</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in visible" :key="e.name" class="border-t border-border first:border-t-0">
              <td class="px-4 py-1.5 font-mono text-foreground break-all">{{ e.name }}</td>
              <td class="px-4 py-1.5 text-right font-mono tabular-nums text-muted">{{ formatBytes(e.size) }}</td>
              <td class="px-4 py-1.5 text-right font-mono tabular-nums text-muted">{{ formatBytes(e.compressed_size) }}</td>
              <td class="px-4 py-1.5 font-mono text-muted">{{ crc(e.crc32) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="visible.length === 0" class="px-4 py-3 text-sm text-muted">{{ t('android.files.noMatch') }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { t, tDynamic } from '../../../i18n/en'
import { chartColor } from '../../../lib/formats'
import { entryCategory, formatBytes } from '../lib'
import type { VersionDetail } from '../types'

const props = defineProps<{ detail: VersionDetail }>()

const filter = ref('')
const entries = computed(() => props.detail.version.entries ?? [])

const visible = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return (q ? entries.value.filter((e) => e.name.toLowerCase().includes(q)) : entries.value).slice(0, 2000)
})

const categories = computed(() => {
  const by = new Map<string, { id: string; size: number; count: number }>()
  for (const e of entries.value) {
    const id = entryCategory(e.name)
    const c = by.get(id) ?? { id, size: 0, count: 0 }
    c.size += e.size
    c.count++
    by.set(id, c)
  }
  return [...by.values()].sort((a, b) => b.size - a.size)
})

const total = computed(() => categories.value.reduce((n, c) => n + c.size, 0))

function crc(v: number): string {
  return v.toString(16).padStart(8, '0')
}
</script>
