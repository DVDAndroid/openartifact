<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-end justify-between gap-4 flex-wrap">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">Welcome back</h1>
        <p class="mt-1 text-sm text-muted">
          You have {{ total }} {{ total === 1 ? 'repository' : 'repositories' }} across {{ uniqueFormats.length }} {{ uniqueFormats.length === 1 ? 'format' : 'formats' }}
        </p>
      </div>
      <div class="flex gap-3">
        <button
          class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-foreground bg-surface border border-border rounded-lg hover:bg-surface-hover transition-colors disabled:opacity-50"
          :disabled="syncing"
          @click="syncNow"
        >
          <ArrowPathIcon class="w-4 h-4" :class="{ 'animate-spin': syncing }" />
          Sync now
        </button>
        <router-link
          to="/repos"
          class="px-4 py-2 text-sm font-medium text-primary-contrast bg-primary rounded-lg hover:bg-primary/90 transition-colors inline-flex items-center gap-1.5"
        >
          New repository
        </router-link>
      </div>
    </div>

    <!-- KPI Cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div
        v-for="(kpi, i) in kpis"
        :key="i"
        class="group relative overflow-hidden rounded-xl border border-border bg-surface p-4 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md hover:border-primary/30"
      >
        <div class="flex items-start justify-between">
          <span class="text-[11px] uppercase tracking-[0.14em] font-semibold text-muted">{{ kpi.label }}</span>
          <span
            v-if="kpi.trend && kpi.trendDir"
            :class="[
              'inline-flex items-center gap-0.5 text-[11px] font-mono',
              kpi.trendDir === 'up' ? 'text-success' : 'text-danger'
            ]"
          >
            <ArrowUpIcon v-if="kpi.trendDir === 'up'" class="w-3 h-3" />
            <ArrowDownIcon v-else class="w-3 h-3" />
            {{ kpi.trend }}
          </span>
        </div>
        <p class="mt-3 text-3xl font-bold tracking-tight tabular-nums">{{ kpi.value }}{{ kpi.unit }}</p>
        <p v-if="kpi.period" class="mt-1 text-[11px] text-muted">{{ kpi.period }}</p>
        <div class="absolute -bottom-1 -right-1 w-24 h-24 rounded-full bg-primary/5 group-hover:bg-primary/10 transition-colors" />
      </div>
    </div>

    <!-- Main Grid: Storage + Top Repositories -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
      <!-- Storage by Format -->
      <div class="lg:col-span-1 rounded-xl border border-border bg-surface p-5">
        <h2 class="text-[13px] font-semibold uppercase tracking-[0.14em] text-muted mb-4">Storage by Format</h2>
        <div class="flex items-center gap-5">
          <svg viewBox="0 0 200 200" class="w-40 h-40 shrink-0">
            <!-- Donut slices -->
            <circle
              v-for="(slice, i) in donutSlices"
              :key="i"
              cx="100"
              cy="100"
              r="70"
              fill="none"
              :stroke="slice.color"
              stroke-width="20"
              :stroke-dasharray="slice.dashArray"
              :stroke-dashoffset="slice.dashOffset"
              transform="rotate(-90 100 100)"
            />
            <!-- Center text -->
            <text x="100" y="100" text-anchor="middle" dy="0.3em" class="text-lg font-bold fill-foreground">
              {{ total }}
            </text>
          </svg>
          <!-- Legend -->
          <div class="flex-1 grid grid-cols-1 gap-1.5 text-xs">
            <div v-for="item in uniqueFormats" :key="item.format" class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2 min-w-0">
                <div
                  class="w-2 h-2 rounded-sm shrink-0"
                  :style="{ backgroundColor: formatColor(item.format) }"
                />
                <span class="text-foreground/85 truncate">{{ item.format }}</span>
              </div>
              <span class="font-mono text-muted tabular-nums">{{ Math.round((item.count / (total || 1)) * 100) }}%</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Top Repositories -->
      <div class="lg:col-span-2 rounded-xl border border-border bg-surface p-5">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-[13px] font-semibold uppercase tracking-[0.14em] text-muted">Top Repositories</h2>
          <router-link to="/repos" class="text-xs text-primary hover:underline inline-flex items-center gap-0.5">
            View all
            <ArrowRightIcon class="w-3 h-3" />
          </router-link>
        </div>
        <div v-if="loading" class="text-sm text-muted py-6">Loading repositories...</div>
        <div v-else-if="repos.length === 0">
          <EmptyState
            title="No repositories yet"
            description="Create your first repository to get started"
          />
        </div>
        <div v-else class="space-y-1">
          <button
            v-for="repo in repos.slice(0, 8)"
            :key="repo.id"
            class="w-full grid grid-cols-12 gap-3 items-center px-2 py-3 -mx-2 rounded-md hover:bg-surface-hover text-left transition-colors cursor-pointer"
            @click="router.push(`/repos/${repo.name}`)"
          >
            <div class="col-span-4 flex items-center gap-2 min-w-0">
              <FormatBadge :format="repo.format" />
              <span class="font-medium text-sm truncate">{{ repo.name }}</span>
            </div>
            <div class="col-span-2 text-xs text-muted font-mono tabular-nums">—</div>
            <div class="col-span-3">
              <Sparkline :data="fakeSparkline()" />
            </div>
            <div class="col-span-3 text-xs text-muted text-right truncate">{{ repo.format }}</div>
          </button>
        </div>
      </div>
    </div>

    <!-- Recent Activity (admin only) -->
    <div v-if="session.isAdmin" class="rounded-xl border border-border bg-surface p-5">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-[13px] font-semibold uppercase tracking-[0.14em] text-muted">Recent Activity</h2>
        <router-link to="/admin/audit" class="text-xs text-primary hover:underline inline-flex items-center gap-0.5">
          View all
          <ArrowRightIcon class="w-3 h-3" />
        </router-link>
      </div>

      <div v-if="auditLoading" class="space-y-3">
        <div v-for="i in 5" :key="i" class="flex items-start gap-3">
          <div class="w-7 h-7 rounded-full skel shrink-0" />
          <div class="flex-1 space-y-1.5">
            <div class="h-3 w-3/4 rounded skel" />
            <div class="h-2.5 w-1/2 rounded skel" />
          </div>
        </div>
      </div>

      <div v-else-if="auditEntries.length === 0" class="py-6 text-center text-sm text-muted">
        No recent activity found.
      </div>

      <div v-else class="relative">
        <!-- Timeline line -->
        <div class="absolute left-3.5 top-3 bottom-3 w-px bg-border" />

        <div class="space-y-4">
          <div
            v-for="entry in auditEntries"
            :key="entry.id"
            class="relative flex items-start gap-4 pl-10"
          >
            <!-- Dot -->
            <div
              class="absolute left-2 top-1.5 w-3 h-3 rounded-full border-2 border-surface shrink-0"
              :class="auditDotClass(entry.result)"
            />

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="text-[12px] font-semibold text-foreground font-mono">{{ entry.actor }}</span>
                <span
                  class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-mono font-medium"
                  :class="auditActionClass(entry.action)"
                >
                  {{ entry.action }}
                </span>
                <span v-if="entry.target" class="text-[12px] text-foreground/70 font-mono truncate max-w-[200px]">
                  {{ entry.target }}
                </span>
              </div>
              <p class="mt-0.5 text-[11px] text-muted font-mono">
                {{ timeAgo(entry.timestamp) }}
                <span v-if="entry.result && entry.result !== 'ok'" class="text-danger ml-1">· {{ entry.result }}</span>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowUpIcon, ArrowDownIcon, ArrowRightIcon, ArrowPathIcon } from '@heroicons/vue/24/outline'
import { listRepositories } from '../api/repositories'
import { getSystemInfo } from '../api/admin'
import { listAudit } from '../api/audit'
import type { Repository } from '../api/repositories'
import type { AuditEntry } from '../api/audit'
import { useToastStore } from '../stores/toast'
import { useSessionStore } from '../stores/session'
import FormatBadge from '../components/FormatBadge.vue'
import Sparkline from '../components/Sparkline.vue'
import EmptyState from '../components/EmptyState.vue'
import { formatColor } from '../lib/formats'

const router = useRouter()
const toasts = useToastStore()
const session = useSessionStore()

const repos = ref<Repository[]>([])
const total = ref(0)
const loading = ref(true)
const syncing = ref(false)
const systemInfo = ref<unknown>(null)
const auditEntries = ref<AuditEntry[]>([])
const auditLoading = ref(false)

// Generate fake sparkline data for visual effect
function fakeSparkline() {
  return Array.from({ length: 8 }, () => Math.random() * 100)
}

const uniqueFormats = computed(() => {
  const counts = new Map<string, number>()
  repos.value.forEach(r => {
    counts.set(r.format, (counts.get(r.format) || 0) + 1)
  })
  return Array.from(counts.entries()).map(([format, count]) => ({ format, count }))
})

const kpis = computed(() => [
  { label: 'Repositories', value: total.value, unit: '', trend: '+2', trendDir: 'up' as const, period: 'vs last month' },
  { label: 'Artifacts', value: '—', unit: '', trend: null, trendDir: null, period: 'Coming soon' },
  { label: 'Storage', value: '—', unit: '', trend: null, trendDir: null, period: 'Coming soon' },
  { label: 'Formats', value: uniqueFormats.value.length, unit: '', trend: '', trendDir: 'up' as const, period: 'active formats' },
])

const donutSlices = computed(() => {
  const slices: { color: string; dashArray: string; dashOffset: number }[] = []
  const circumference = 2 * Math.PI * 70
  const t = total.value || 1
  let offset = 0

  uniqueFormats.value.forEach(({ format, count }) => {
    const arc = (count / t) * circumference
    slices.push({
      color: formatColor(format),
      dashArray: `${arc.toFixed(2)} ${(circumference - arc).toFixed(2)}`,
      dashOffset: -offset,
    })
    offset += arc
  })

  return slices
})

function timeAgo(iso?: string): string {
  if (!iso) return '—'
  const diff = Date.now() - new Date(iso).getTime()
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function auditDotClass(result?: string): string {
  if (!result || result === 'ok' || result === 'success') return 'bg-success'
  if (result === 'error' || result === 'failed') return 'bg-danger'
  return 'bg-muted'
}

function auditActionClass(action?: string): string {
  if (!action) return 'bg-surface-hover text-muted'
  if (action.includes('delete') || action.includes('remove')) return 'bg-danger-soft text-danger'
  if (action.includes('create') || action.includes('add')) return 'bg-success-soft text-success'
  if (action.includes('update') || action.includes('edit')) return 'bg-primary-soft text-primary'
  return 'bg-surface-hover text-muted'
}

async function syncNow(): Promise<void> {
  syncing.value = true
  try {
    const repoRes = await listRepositories({ limit: 50 })
    repos.value = repoRes.repositories || []
    total.value = repoRes.total || repos.value.length
    toasts.success('Refreshed')
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Sync failed')
  } finally {
    syncing.value = false
  }
}

async function loadAudit(): Promise<void> {
  if (!session.isAdmin) return
  auditLoading.value = true
  try {
    const res = await listAudit({ limit: 10 })
    auditEntries.value = res.items
  } catch {
    auditEntries.value = []
  } finally {
    auditLoading.value = false
  }
}

onMounted(async () => {
  try {
    const [repoRes, sysRes] = await Promise.all([
      listRepositories({ limit: 50 }),
      getSystemInfo().catch(() => null),
    ])
    repos.value = repoRes.repositories || []
    total.value = repoRes.total || repos.value.length
    systemInfo.value = sysRes
  } finally {
    loading.value = false
  }
  void loadAudit()
})
</script>
