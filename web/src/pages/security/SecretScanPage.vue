<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between gap-4 flex-wrap">
      <div>
        <div class="flex items-center gap-1.5 text-[11px] uppercase tracking-[0.14em] text-muted font-mono">
          <span>Security</span>
          <ChevronRightIcon class="w-3 h-3" />
          <span class="text-foreground/70">Secrets</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">Secret Scanning</h1>
        <p class="mt-1 text-sm text-muted">Detect accidentally committed secrets, API keys, and credentials in your artifacts.</p>
      </div>
      <button
        class="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50"
        :disabled="triggering || !selectedRepo"
        @click="triggerScan"
      >
        <ArrowPathIcon class="w-4 h-4" :class="{ 'animate-spin': triggering }" />
        Re-scan
      </button>
    </div>

    <!-- Repo selector -->
    <div class="flex items-center gap-3">
      <select
        v-model="selectedRepo"
        class="h-9 px-3 rounded-lg bg-surface border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
        @change="loadScans"
      >
        <option value="">Select a repository…</option>
        <option v-for="r in repos" :key="r.name" :value="r.name">{{ r.name }}</option>
      </select>
      <button
        class="flex items-center justify-center w-9 h-9 rounded-lg border border-border text-muted hover:text-foreground hover:bg-surface-hover transition-colors disabled:opacity-50"
        :disabled="loadingScans || !selectedRepo"
        @click="loadScans"
      >
        <ArrowPathIcon class="w-4 h-4" :class="{ 'animate-spin': loadingScans }" />
      </button>
    </div>

    <!-- Stats tiles -->
    <div class="grid grid-cols-3 gap-4">
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Open</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-danger">{{ openCount }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Allowlisted</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-warning">{{ allowlistedCount }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Resolved</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-success">{{ resolvedCount }}</div>
      </div>
    </div>

    <!-- Status tabs -->
    <div class="flex items-center gap-1 bg-surface-hover/60 rounded-lg p-0.5 w-fit">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
        :class="activeTab === tab.id ? 'bg-surface text-foreground shadow-sm' : 'text-muted hover:text-foreground'"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
        <span class="ml-1 font-mono text-[10px]">({{ tab.count }})</span>
      </button>
    </div>

    <!-- Filter toolbar -->
    <div class="flex items-center gap-3 flex-wrap">
      <div class="relative flex-1 max-w-sm">
        <MagnifyingGlassIcon class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
        <input
          v-model="searchQuery"
          type="search"
          placeholder="Search scans…"
          class="w-full pl-9 pr-3 h-9 text-sm rounded-lg bg-surface-hover/40 border border-border text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
        />
      </div>
    </div>

    <!-- Scans table -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Secret Scans</h3>
        <span class="text-[11px] font-mono text-muted">{{ filteredScans.length }} scans</span>
      </div>

      <div v-if="loadingScans" class="py-12 flex flex-col items-center gap-3 text-muted">
        <ArrowPathIcon class="w-6 h-6 animate-spin" />
        <span class="text-sm">Loading scans…</span>
      </div>

      <EmptyState
        v-else-if="!selectedRepo"
        title="Select a repository"
        description="Choose a repository above to view its secret scans."
      />

      <EmptyState
        v-else-if="filteredScans.length === 0"
        title="No secret scans found"
        description="Trigger a scan using the button above to detect exposed secrets."
      />

      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="border-b border-border bg-surface-hover/40">
            <tr>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Scan ID</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Status</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Findings</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Started At</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="scan in filteredScans"
              :key="scan.id"
              class="hover:bg-surface-hover/40 transition-colors border-b border-border last:border-0"
            >
              <td class="px-4 py-3 font-mono text-[13px] text-muted">{{ scan.id?.slice(0, 8) }}…</td>
              <td class="px-4 py-3">
                <span :class="scanStatusClass(scan.status || '')">{{ scan.status }}</span>
              </td>
              <td class="px-4 py-3 font-mono text-[13px] text-muted">{{ scan.finding_count ?? '—' }}</td>
              <td class="px-4 py-3 text-xs text-muted font-mono">{{ formatDate(scan.created_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1.5">
                  <button
                    disabled
                    class="px-2 py-1 rounded-md text-[11px] font-medium border border-border text-muted hover:bg-surface-hover transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                  >Allowlist</button>
                  <button
                    disabled
                    class="px-2 py-1 rounded-md text-[11px] font-medium border border-border text-muted hover:bg-surface-hover transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                  >Resolve</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ChevronRightIcon, ArrowPathIcon, MagnifyingGlassIcon } from '@heroicons/vue/24/outline'
import EmptyState from '../../components/EmptyState.vue'
import { useToastStore } from '../../stores/toast'

const toasts = useToastStore()

interface SecretScan {
  id?: string
  status?: string
  finding_count?: number
  created_at?: string
}

const repos = ref<any[]>([])
const selectedRepo = ref('')
const scans = ref<SecretScan[]>([])
const loadingScans = ref(false)
const triggering = ref(false)
const searchQuery = ref('')
const activeTab = ref('all')

const openCount = computed(() => scans.value.filter(s => s.status === 'open').length)
const allowlistedCount = computed(() => scans.value.filter(s => s.status === 'allowlisted').length)
const resolvedCount = computed(() => scans.value.filter(s => s.status === 'resolved').length)

const tabs = computed(() => [
  { id: 'all', label: 'All', count: scans.value.length },
  { id: 'open', label: 'Open', count: openCount.value },
  { id: 'allowlisted', label: 'Allowlisted', count: allowlistedCount.value },
  { id: 'resolved', label: 'Resolved', count: resolvedCount.value },
])

const filteredScans = computed(() => {
  let result = scans.value
  const q = searchQuery.value.trim().toLowerCase()
  if (q) result = result.filter(s => (s.id || '').toLowerCase().includes(q))
  if (activeTab.value !== 'all') result = result.filter(s => s.status === activeTab.value)
  return result
})

function scanStatusClass(status: string): string {
  const map: Record<string, string> = {
    open: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-danger bg-danger-soft/60',
    allowlisted: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-warning bg-warning-soft/60',
    resolved: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-success bg-success-soft/60',
    pending: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-warning bg-warning-soft/60',
    completed: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-success bg-success-soft/60',
    failed: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-danger bg-danger-soft/60',
  }
  return map[status] || map.pending
}

function formatDate(ts?: string): string {
  if (!ts) return '—'
  try { return new Date(ts).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) } catch { return ts }
}

async function loadRepos(): Promise<void> {
  try {
    const res = await fetch('/api/v1/repositories?limit=100', { credentials: 'include' })
    const data = await res.json()
    repos.value = data.repositories || []
  } catch { /* ignore */ }
}

async function loadScans(): Promise<void> {
  if (!selectedRepo.value) return
  loadingScans.value = true
  try {
    const res = await fetch(`/api/v1/repositories/${encodeURIComponent(selectedRepo.value)}/secrets/scans`, { credentials: 'include' })
    const data = await res.json()
    scans.value = data.scans || []
  } catch {
    toasts.error('Failed to load secret scans')
  } finally {
    loadingScans.value = false
  }
}

async function triggerScan(): Promise<void> {
  if (!selectedRepo.value) return
  triggering.value = true
  try {
    const res = await fetch(`/api/v1/repositories/${encodeURIComponent(selectedRepo.value)}/secrets/scan`, {
      method: 'POST',
      credentials: 'include',
    })
    if (!res.ok) throw new Error()
    toasts.success('Secret scan started')
    await loadScans()
  } catch {
    toasts.error('Failed to trigger secret scan')
  } finally {
    triggering.value = false
  }
}

onMounted(loadRepos)
</script>
