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
          <span class="text-foreground/70">{{ t('nav.tokens') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.tokens.title') }}</h1>
        <p class="mt-1 text-sm text-muted">Manage API tokens across the system</p>
      </div>
    </div>

    <!-- StatRow -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft">
          <KeyIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Total Tokens</div>
          <div class="text-xl font-bold tabular-nums">{{ total }}</div>
        </div>
      </div>
    </div>

    <!-- TableCard -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Tokens</h3>
        <span class="text-[11px] font-mono text-muted">{{ tokens.length }} total</span>
      </div>
      <div v-if="loading" class="px-4 py-8 text-center text-muted">Loading...</div>
      <div v-else-if="tokens.length === 0" class="px-4 py-8 text-center text-muted">No API tokens found</div>
      <div v-else class="overflow-x-auto">
        <DataTable :columns="columns" :rows="tokens as Record<string, unknown>[]" row-key="id">
          <template #cell-name="{ value }">
            <span class="font-mono text-[13px] font-medium">{{ value }}</span>
          </template>
          <template #cell-user_id="{ value }">
            <span class="text-sm text-muted font-mono">{{ value || '—' }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-xs text-muted font-mono">{{ formatDate(value as string) }}</span>
          </template>
          <template #cell-last_used_at="{ value }">
            <span class="text-xs text-muted font-mono">{{ formatDate(value as string) || 'Never' }}</span>
          </template>
          <template #cell-expires_at="{ value }">
            <span :class="['text-xs font-mono', isExpired(value as string) ? 'text-danger' : 'text-muted']">
              {{ formatDate(value as string) || 'Never' }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <button
              @click="revoke(row.id as string)"
              class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-medium text-danger hover:bg-danger/10 transition-colors"
            >
              <TrashIcon class="w-3.5 h-3.5" />
              Revoke
            </button>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="tokens.length" class="flex items-center justify-between">
      <span class="text-sm text-muted">Page {{ page }} of {{ Math.ceil(total / limit) }}</span>
      <div class="flex items-center gap-2">
        <button
          @click="previousPage"
          :disabled="page === 1"
          class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Previous
        </button>
        <button
          @click="nextPage"
          :disabled="page >= Math.ceil(total / limit)"
          class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Next
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import DataTable, { type Column } from '../../components/DataTable.vue'

const columns: Column[] = [
  { key: 'name', label: 'Name' },
  { key: 'user_id', label: 'Owner' },
  { key: 'created_at', label: 'Created' },
  { key: 'last_used_at', label: 'Last Used' },
  { key: 'expires_at', label: 'Expires' },
  { key: 'actions', label: 'Actions' }
]
import { listTokens, revokeToken, type AdminToken } from '../../api/tokens'
type ApiToken = AdminToken & { id: string }
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import { ChevronRightIcon, KeyIcon, TrashIcon } from '@heroicons/vue/24/outline'

const tokens = ref<ApiToken[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const limit = ref(25)
const toasts = useToastStore()

function formatDate(date?: string): string {
  if (!date) return ''
  try {
    return new Date(date).toLocaleDateString('en-US', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
  } catch {
    return date
  }
}

function isExpired(date?: string): boolean {
  if (!date) return false
  try {
    return new Date(date) < new Date()
  } catch {
    return false
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listTokens({ page: page.value, limit: limit.value })
    tokens.value = res.items as ApiToken[]
    total.value = res.total
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load tokens')
  } finally {
    loading.value = false
  }
}

async function revoke(id: string | number): Promise<void> {
  if (!window.confirm('Revoke this token?')) return
  try {
    await revokeToken(id)
    toasts.success('Token revoked')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to revoke token')
  }
}

function previousPage(): void {
  if (page.value > 1) {
    page.value--
    load()
  }
}

function nextPage(): void {
  if (page.value < Math.ceil(total.value / limit.value)) {
    page.value++
    load()
  }
}

onMounted(load)
</script>
