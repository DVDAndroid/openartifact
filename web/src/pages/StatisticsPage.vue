<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div>
    <!-- Title -->
    <h1 class="text-2xl font-semibold text-foreground">Statistics</h1>

    <!-- KPI Cards -->
    <div class="grid grid-cols-4 gap-4 mt-6">
      <div
        v-for="(kpi, idx) in kpiCards"
        :key="idx"
        class="bg-surface rounded-xl border border-border p-5 relative overflow-hidden animate-fade-in"
        :style="{ animationDelay: `${idx * 100}ms` }"
      >
        <div class="absolute -top-4 -right-4 w-16 h-16 rounded-full bg-primary/5" />
        <p class="text-xs text-muted uppercase tracking-wide">{{ kpi.label }}</p>
        <p class="text-2xl font-bold mt-1">{{ kpi.value }}</p>
      </div>
    </div>

    <!-- Main Content: 2-column layout -->
    <div class="grid grid-cols-5 gap-6 mt-6">
      <!-- Left Column (3 cols): Repositories by Format -->
      <div class="bg-surface rounded-xl border border-border p-6 col-span-3">
        <h2 class="text-sm font-semibold">Repositories by Format</h2>
        <div v-if="loading" class="space-y-4 mt-4">
          <div v-for="n in 5" :key="n" class="space-y-2">
            <div class="h-4 bg-surface-hover rounded animate-pulse w-3/4" />
            <div class="h-2 bg-surface-hover rounded animate-pulse" />
          </div>
        </div>
        <div v-else-if="formatCounts.size === 0" class="text-muted text-sm py-4 mt-4">
          No repositories
        </div>
        <div v-else class="mt-4 space-y-3">
          <div
            v-for="item in formatCountsArray"
            :key="item.format"
            class="space-y-1"
          >
            <div class="flex items-center justify-between">
              <span class="text-sm text-foreground">{{ item.format }}</span>
              <span class="text-sm text-muted">{{ item.count }}</span>
            </div>
            <div class="h-2 rounded-full bg-surface-hover overflow-hidden">
              <div
                class="h-full rounded-full bar-fill transition-all duration-500"
                :style="{
                  width: `${(item.count / (formatCountsArray[0]?.count || 1)) * 100}%`,
                  backgroundColor: formatColor(item.format),
                }"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- Right Column (2 cols): Format Distribution Donut -->
      <div class="bg-surface rounded-xl border border-border p-6 col-span-2">
        <h2 class="text-sm font-semibold">Format Distribution</h2>
        <div v-if="loading" class="flex items-center justify-center h-64">
          <div class="w-32 h-32 bg-surface-hover rounded-full animate-pulse" />
        </div>
        <div v-else-if="repos.length === 0" class="flex items-center justify-center h-64 text-muted">
          No data
        </div>
        <div v-else class="mt-4 flex flex-col items-center">
          <!-- Donut Chart -->
          <svg viewBox="0 0 180 180" class="w-32 h-32">
            <circle
              cx="90"
              cy="90"
              r="60"
              fill="none"
              stroke-width="18"
              stroke="rgb(var(--color-surface-hover))"
            />
            <circle
              v-for="slice of donutSlices"
              :key="slice.format"
              cx="90"
              cy="90"
              r="60"
              fill="none"
              :stroke="slice.color"
              stroke-width="18"
              :stroke-dasharray="slice.dashArray"
              :stroke-dashoffset="slice.dashOffset"
              stroke-linecap="round"
              class="transition-all duration-500"
            />
            <!-- Center text -->
            <text x="90" y="85" text-anchor="middle" class="text-sm font-bold fill-foreground">
              {{ repos.length }}
            </text>
            <text x="90" y="100" text-anchor="middle" class="text-xs fill-muted">
              repos
            </text>
          </svg>

          <!-- Legend -->
          <div class="grid grid-cols-2 gap-2 mt-4 w-full">
            <div
              v-for="item in formatCountsArray"
              :key="item.format"
              class="flex items-center gap-2 text-xs"
            >
              <div
                class="w-2 h-2 rounded-full shrink-0"
                :style="{ backgroundColor: formatColor(item.format) }"
              />
              <span class="text-foreground">{{ item.format }}</span>
              <span class="text-muted ml-auto">{{ item.count }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Repository Table -->
    <div class="bg-surface rounded-xl border border-border mt-6 overflow-hidden">
      <div class="px-6 py-4 bg-surface-hover/50">
        <h2 class="text-sm font-semibold">All Repositories</h2>
      </div>

      <div v-if="loading" class="px-6 py-8 text-muted text-sm text-center">
        Loading...
      </div>

      <EmptyState
        v-else-if="repos.length === 0"
        title="No repositories"
        description="Create your first repository to get started"
      />

      <table v-else class="w-full text-sm">
        <thead>
          <tr class="bg-surface-hover/50 text-xs text-muted uppercase tracking-wider border-b border-border">
            <th scope="col" class="text-left px-6 py-3 font-semibold">Name</th>
            <th scope="col" class="text-left px-6 py-3 font-semibold">Format</th>
            <th scope="col" class="text-left px-6 py-3 font-semibold">Type</th>
            <th scope="col" class="text-left px-6 py-3 font-semibold">Description</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="repo in repos"
            :key="repo.id"
            class="border-t border-border hover:bg-surface-hover transition-colors"
          >
            <td class="px-6 py-3">
              <router-link
                :to="`/repos/${repo.name}`"
                class="font-medium text-foreground hover:text-primary"
              >
                {{ repo.name }}
              </router-link>
            </td>
            <td class="px-6 py-3">
              <FormatBadge :format="repo.format" />
            </td>
            <td class="px-6 py-3">
              <span
                class="inline-block px-2 py-1 rounded text-xs font-medium"
                :class="repo.public ? 'bg-success/10 text-success' : 'bg-surface-hover text-muted'"
              >
                {{ repo.public ? 'Public' : 'Private' }}
              </span>
            </td>
            <td class="px-6 py-3 text-sm text-muted truncate">
              {{ repo.description || '—' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { listRepositories } from '../api/repositories'
import type { Repository } from '../api/repositories'
import FormatBadge from '../components/FormatBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { formatColor } from '../lib/formats'

const router = useRouter()
const repos = ref<Repository[]>([])
const loading = ref(true)

// Computed format counts (sorted by count descending)
const formatCounts = computed(() => {
  const counts = new Map<string, number>()
  repos.value.forEach(r => counts.set(r.format, (counts.get(r.format) || 0) + 1))
  const entries: Array<[string, number]> = []
  counts.forEach((count, format) => {
    entries.push([format, count])
  })
  entries.sort((a, b) => b[1] - a[1])
  return new Map(entries)
})

// Format counts as array for template iteration
const formatCountsArray = computed(() => {
  const arr: Array<{ format: string; count: number }> = []
  formatCounts.value.forEach((count: number, format: string) => {
    arr.push({ format, count })
  })
  return arr
})

// KPI Cards
const kpiCards = computed(() => [
  {
    label: 'Total Repositories',
    value: repos.value.length,
  },
  {
    label: 'Public Repos',
    value: repos.value.filter(r => r.public).length,
  },
  {
    label: 'Private Repos',
    value: repos.value.filter(r => !r.public).length,
  },
  {
    label: 'Formats',
    value: formatCounts.value.size,
  },
])

// Donut chart slices
const donutSlices = computed(() => {
  const circumference = 2 * Math.PI * 60
  const total = repos.value.length || 1
  let currentOffset = 0
  const slices: Array<{
    format: string
    count: number
    color: string
    dashArray: string
    dashOffset: number
  }> = []

  formatCountsArray.value.forEach(({ format, count }) => {
    const pct = count / total
    const dash = circumference * pct
    slices.push({
      format,
      count,
      color: formatColor(format),
      dashArray: `${dash} ${circumference - dash}`,
      dashOffset: -currentOffset,
    })
    currentOffset += dash
  })

  return slices
})

// Load repositories
onMounted(async () => {
  try {
    const res = await listRepositories({ limit: 200 })
    repos.value = res.repositories || []
  } finally {
    loading.value = false
  }
})
</script>
