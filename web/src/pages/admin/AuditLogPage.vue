<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-5">
    <!-- PageHead -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <div class="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-muted font-mono">
          <span>{{ t('nav.admin') }}</span>
          <ChevronRightIcon class="w-3 h-3" />
          <span class="text-foreground/70">{{ t('nav.audit') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.audit.title') }}</h1>
        <p class="mt-1 text-sm text-muted">Immutable record of administrative actions</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          disabled
          class="px-3 py-1.5 text-sm rounded-lg bg-surface border border-border text-muted hover:opacity-90 transition-opacity font-medium disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Export
        </button>
      </div>
    </div>

    <!-- Filter Panel -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-3">
      <div>
        <label class="block text-[11px] uppercase tracking-[0.14em] text-muted font-semibold mb-1.5">{{ t('admin.audit.filter.actor') }}</label>
        <input
          v-model="filters.actor"
          type="text"
          placeholder="Filter by actor..."
          class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
        />
      </div>
      <div>
        <label class="block text-[11px] uppercase tracking-[0.14em] text-muted font-semibold mb-1.5">{{ t('admin.audit.filter.action') }}</label>
        <select
          v-model="filters.action"
          class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
        >
          <option value="">All actions</option>
          <option v-for="action in uniqueActions" :key="action" :value="action">
            {{ action }}
          </option>
        </select>
      </div>
      <div>
        <label class="block text-[11px] uppercase tracking-[0.14em] text-muted font-semibold mb-1.5">{{ t('admin.audit.filter.from') }}</label>
        <input
          v-model="filters.from"
          type="date"
          class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
        />
      </div>
      <div>
        <label class="block text-[11px] uppercase tracking-[0.14em] text-muted font-semibold mb-1.5">{{ t('admin.audit.filter.to') }}</label>
        <input
          v-model="filters.to"
          type="date"
          class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
        />
      </div>
    </div>

    <!-- TableCard -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Audit Entries</h3>
        <span class="text-[11px] font-mono text-muted">{{ total }} total</span>
      </div>
      <div v-if="loading" class="px-4 py-8 text-center text-muted">Loading...</div>
      <div v-else-if="entries.length === 0" class="px-4 py-8 text-center text-muted">No audit entries found</div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-[11px] uppercase tracking-[0.12em] font-semibold text-muted bg-surface-hover/30">
              <th class="text-left px-4 py-2.5 font-medium w-40">{{ t('admin.audit.column.timestamp') }}</th>
              <th class="text-left px-4 py-2.5 font-medium w-32">{{ t('admin.audit.column.actor') }}</th>
              <th class="text-left px-4 py-2.5 font-medium w-28">{{ t('admin.audit.column.action') }}</th>
              <th class="text-left px-4 py-2.5 font-medium flex-1">{{ t('admin.audit.column.target') }}</th>
              <th class="text-center px-4 py-2.5 font-medium w-10"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <template v-for="entry in entries" :key="entry.id">
              <tr class="hover:bg-surface-hover/40 transition-colors">
                <td class="px-4 py-3">
                  <span class="font-mono text-[12px] text-muted">{{ formatTimestamp(entry.timestamp) }}</span>
                </td>
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <div class="w-6 h-6 rounded-full bg-primary/20 text-primary text-[10px] font-bold flex items-center justify-center flex-shrink-0">
                      {{ (entry.actor || '?')[0].toUpperCase() }}
                    </div>
                    <span class="font-mono text-[13px] font-medium truncate">{{ entry.actor || '—' }}</span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span
                    :class="[
                      'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium',
                      getActionPillClass(entry.action),
                    ]"
                  >
                    {{ entry.action || '—' }}
                  </span>
                </td>
                <td class="px-4 py-3">
                  <span class="text-sm text-muted truncate max-w-[200px] block">{{ entry.target || '—' }}</span>
                </td>
                <td class="px-4 py-3 text-center">
                  <button
                    @click="toggleExpand(entry.id)"
                    :class="[
                      'p-1 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-all',
                      entry.id && expanded.has(entry.id) ? 'rotate-90' : '',
                    ]"
                  >
                    <ChevronRightIcon class="w-4 h-4" />
                  </button>
                </td>
              </tr>
              <tr v-if="entry.id && expanded.has(entry.id)" class="bg-surface-hover/20">
                <td colspan="5" class="px-4 py-3">
                  <div class="space-y-2">
                    <div v-if="entry.result" class="flex gap-2">
                      <span class="text-[11px] text-muted font-semibold min-w-fit">Result:</span>
                      <span class="text-[11px] text-foreground">{{ entry.result }}</span>
                    </div>
                    <div v-if="entry.ip" class="flex gap-2">
                      <span class="text-[11px] text-muted font-semibold min-w-fit">IP:</span>
                      <span class="text-[11px] text-foreground font-mono">{{ entry.ip }}</span>
                    </div>
                    <div v-if="entry.details" class="flex flex-col gap-1">
                      <span class="text-[11px] text-muted font-semibold">Details:</span>
                      <pre class="font-mono text-[11px] bg-surface-hover rounded-lg p-3 overflow-x-auto text-foreground/80">{{ JSON.stringify(entry.details, null, 2) }}</pre>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="entries.length" class="flex items-center justify-between">
      <span class="text-sm text-muted">Page {{ filters.page }} of {{ Math.ceil(total / (filters.limit || 25)) }}</span>
      <div class="flex items-center gap-2">
        <button
          @click="previousPage"
          :disabled="filters.page === 1"
          class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Previous
        </button>
        <button
          @click="nextPage"
          :disabled="(filters.page ?? 1) >= Math.ceil(total / (filters.limit || 25))"
          class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Next
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { listAudit, type AuditEntry, type AuditFilters } from '../../api/audit'
import { ChevronRightIcon } from '@heroicons/vue/24/outline'
import { t } from '../../i18n/en'

const entries = ref<AuditEntry[]>([])
const total = ref(0)
const loading = ref(true)
const expanded = ref<Set<string>>(new Set())

const filters = ref<AuditFilters>({ page: 1, limit: 25 })

const uniqueActions = computed(() => {
  const actions = new Set<string>()
  entries.value.forEach((e) => {
    if (e.action) actions.add(e.action)
  })
  return Array.from(actions).sort()
})

function toggleExpand(id: string | undefined): void {
  if (id == null) return
  const s = new Set(expanded.value)
  s.has(id) ? s.delete(id) : s.add(id)
  expanded.value = s
}

function formatTimestamp(ts: string | undefined): string {
  if (!ts) return '—'
  try {
    return new Date(ts).toLocaleString('en-US', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return ts
  }
}

function getActionPillClass(action?: string): string {
  const base = 'bg-soft text-'
  if (!action) return 'bg-surface-hover/40 text-muted'
  if (action.toLowerCase().includes('create')) return 'bg-success-soft/40 text-success'
  if (action.toLowerCase().includes('delete')) return 'bg-danger-soft/40 text-danger'
  if (action.toLowerCase().includes('update')) return 'bg-warning-soft/40 text-warning'
  return 'bg-primary-soft/40 text-primary'
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listAudit(filters.value)
    entries.value = res.items || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

function previousPage(): void {
  if (filters.value.page && filters.value.page > 1) {
    filters.value.page--
  }
}

function nextPage(): void {
  const maxPage = Math.ceil(total.value / (filters.value.limit || 25))
  if (!filters.value.page || filters.value.page < maxPage) {
    filters.value.page = (filters.value.page || 1) + 1
  }
}

let debounceHandle: ReturnType<typeof setTimeout> | null = null
watch(
  filters,
  () => {
    if (debounceHandle) clearTimeout(debounceHandle)
    debounceHandle = setTimeout(() => {
      load()
    }, 300)
  },
  { deep: true }
)

onMounted(load)
</script>
