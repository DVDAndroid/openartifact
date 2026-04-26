<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-5">

    <!-- ─── Page Head ───────────────────────────────────────────────────────── -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <div class="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-muted font-mono">
          <span>{{ t('nav.admin') }}</span>
          <ChevronRightIcon class="w-3 h-3" />
          <span class="text-foreground/70">Backup &amp; Restore</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">Backup &amp; Restore</h1>
        <p class="mt-1 text-sm text-muted">Create, download and restore system backups.</p>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity shrink-0"
        @click="openCreateModal"
      >
        <PlusIcon class="w-4 h-4" />
        Create backup
      </button>
    </div>

    <!-- ─── Stat Tiles ──────────────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 md:grid-cols-3 gap-3">
      <div
        v-for="tile in statTiles"
        :key="tile.label"
        class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3"
      >
        <div
          class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0"
          :style="{ background: tile.iconBg }"
        >
          <component :is="tile.icon" class="w-5 h-5" :style="{ color: tile.iconColor }" />
        </div>
        <div class="min-w-0">
          <p class="text-[10px] uppercase tracking-[0.12em] font-mono font-semibold text-muted truncate">
            {{ tile.label }}
          </p>
          <p class="text-xl font-bold tabular-nums text-foreground leading-tight">{{ tile.value }}</p>
        </div>
      </div>
    </div>

    <!-- ─── Loading Skeleton ────────────────────────────────────────────────── -->
    <div v-if="loading" class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-5 py-4 border-b border-border">
        <div class="h-4 w-32 rounded skel" />
      </div>
      <div v-for="i in 3" :key="i" class="flex items-center gap-4 px-5 py-4 border-b border-border last:border-0">
        <div class="h-5 w-20 rounded-full skel" />
        <div class="h-5 w-20 rounded-full skel" />
        <div class="h-3 w-24 rounded skel" />
        <div class="h-3 w-28 rounded skel" />
        <div class="flex-1" />
        <div class="flex gap-1">
          <div class="w-7 h-7 rounded-md skel" />
          <div class="w-7 h-7 rounded-md skel" />
        </div>
      </div>
    </div>

    <!-- ─── Table ───────────────────────────────────────────────────────────── -->
    <div v-else class="bg-surface rounded-xl border border-border overflow-hidden">
      <!-- Caption -->
      <div class="px-5 py-4 flex items-center gap-2 border-b border-border">
        <h2 class="text-sm font-semibold text-foreground">Backup jobs</h2>
        <span class="px-1.5 py-0.5 rounded-md bg-surface-hover text-[11px] font-mono font-medium text-muted">
          {{ items.length }}
        </span>
        <!-- Auto-refresh indicator -->
        <span v-if="polling" class="ml-auto flex items-center gap-1.5 text-[11px] text-muted font-mono">
          <span class="w-1.5 h-1.5 rounded-full bg-warning animate-pulse" />
          Auto-refreshing
        </span>
      </div>

      <!-- Empty state -->
      <div v-if="items.length === 0" class="py-16 text-center">
        <div class="w-12 h-12 rounded-full bg-surface-hover mx-auto flex items-center justify-center mb-3">
          <ArchiveBoxIcon class="w-6 h-6 text-muted" />
        </div>
        <p class="text-sm font-medium text-foreground">No backups yet</p>
        <p class="text-xs text-muted mt-1 mb-4">Create your first backup to protect your data.</p>
        <button
          type="button"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
          @click="openCreateModal"
        >
          <PlusIcon class="w-4 h-4" />
          Create backup
        </button>
      </div>

      <!-- Data table -->
      <table v-else class="w-full">
        <thead>
          <tr class="bg-surface-hover/30">
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5">
              Type
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Status
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Size
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Created
            </th>
            <th class="text-right text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-border">
          <tr
            v-for="job in items"
            :key="job.id"
            class="hover:bg-surface-hover/30 transition-colors"
          >
            <!-- Type -->
            <td class="px-5 py-3.5">
              <span
                class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border border-border bg-surface-hover text-[11px] font-mono font-medium text-foreground"
              >
                <component :is="job.type === 'full' ? ServerStackIcon : CircleStackIcon" class="w-3.5 h-3.5 text-muted" />
                {{ job.type === 'full' ? 'Full' : 'Metadata only' }}
              </span>
            </td>

            <!-- Status badge -->
            <td class="px-3 py-3.5">
              <span
                class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium"
                :class="statusBadgeClass(job.status)"
              >
                <span
                  class="w-1.5 h-1.5 rounded-full"
                  :class="[statusDotClass(job.status), job.status === 'running' ? 'animate-pulse' : '']"
                />
                {{ statusLabel(job.status) }}
              </span>
              <p v-if="job.error" class="text-[10px] text-danger font-mono mt-0.5 max-w-[180px] truncate" :title="job.error">
                {{ job.error }}
              </p>
            </td>

            <!-- Size -->
            <td class="px-3 py-3.5 text-sm text-muted font-mono">
              {{ job.size_bytes ? formatBytes(job.size_bytes) : '—' }}
            </td>

            <!-- Created -->
            <td class="px-3 py-3.5">
              <p class="text-sm text-foreground">{{ formatDate(job.created_at) }}</p>
              <p v-if="job.completed_at" class="text-[11px] text-muted font-mono mt-0.5">
                done {{ timeAgo(job.completed_at) }}
              </p>
            </td>

            <!-- Actions -->
            <td class="px-5 py-3.5">
              <div class="flex items-center gap-0.5 justify-end">
                <!-- Download -->
                <a
                  v-if="job.status === 'completed'"
                  :href="`/api/v1/admin/backup/${job.id}/download`"
                  :title="'Download backup'"
                  class="p-1.5 rounded-md text-muted hover:text-primary hover:bg-primary/10 transition-colors"
                  download
                >
                  <ArrowDownTrayIcon class="w-4 h-4" />
                </a>
                <button
                  v-else
                  type="button"
                  disabled
                  class="p-1.5 rounded-md text-muted opacity-30 cursor-not-allowed"
                  title="Available when completed"
                >
                  <ArrowDownTrayIcon class="w-4 h-4" />
                </button>

                <!-- Restore -->
                <button
                  type="button"
                  :disabled="job.status !== 'completed'"
                  title="Restore from this backup"
                  class="p-1.5 rounded-md text-muted hover:text-warning hover:bg-warning/10 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                  @click="openRestoreModal(job)"
                >
                  <ArrowPathIcon class="w-4 h-4" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ═══════════════════════════════════════════════════════════════════════
         CREATE BACKUP MODAL
    ════════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="createModalOpen"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in"
      >
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="closeCreateModal" />
        <div class="relative bg-surface border border-border rounded-xl shadow-2xl w-full max-w-sm">
          <div class="px-6 pt-6 pb-4">
            <div class="flex items-center justify-between gap-3 mb-4">
              <h2 class="text-base font-bold text-foreground tracking-tight">Create backup</h2>
              <button
                type="button"
                class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                @click="closeCreateModal"
              >
                <XMarkIcon class="w-4 h-4" />
              </button>
            </div>

            <!-- Type selector -->
            <div class="space-y-2">
              <label class="block text-xs font-semibold text-foreground mb-2">Backup type</label>
              <button
                v-for="opt in BACKUP_TYPES"
                :key="opt.value"
                type="button"
                class="w-full text-left p-3 rounded-lg border transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                :class="
                  createForm.type === opt.value
                    ? 'border-primary bg-primary/5 ring-1 ring-primary/30'
                    : 'border-border bg-surface hover:bg-surface-hover'
                "
                @click="createForm.type = opt.value as 'full' | 'metadata-only'"
              >
                <div class="flex items-start gap-2.5">
                  <component :is="opt.icon" class="w-4 h-4 mt-0.5 shrink-0" :class="createForm.type === opt.value ? 'text-primary' : 'text-muted'" />
                  <div>
                    <p class="text-[13px] font-semibold text-foreground leading-tight">{{ opt.label }}</p>
                    <p class="text-[11px] text-muted mt-0.5 leading-snug">{{ opt.desc }}</p>
                  </div>
                </div>
              </button>
            </div>
          </div>

          <div class="px-6 pb-6 flex items-center gap-2 justify-end">
            <button
              type="button"
              class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
              @click="closeCreateModal"
            >
              Cancel
            </button>
            <button
              type="button"
              :disabled="creating"
              class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
              @click="confirmCreate"
            >
              <ArrowPathIcon v-if="creating" class="w-4 h-4 animate-spin" />
              <ArchiveBoxArrowDownIcon v-else class="w-4 h-4" />
              Create backup
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ═══════════════════════════════════════════════════════════════════════
         RESTORE CONFIRMATION MODAL
    ════════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="restoreTarget"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in"
      >
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="cancelRestore" />
        <div class="relative bg-surface border border-border rounded-xl shadow-2xl w-full max-w-md">
          <div class="px-6 pt-6 pb-4">
            <h2 class="text-base font-bold text-foreground tracking-tight">Restore from backup</h2>
            <p class="text-sm text-muted mt-0.5">This will overwrite current data. This action cannot be undone.</p>
          </div>

          <div class="px-6 pb-6 space-y-4">
            <!-- Danger alert -->
            <div class="rounded-lg border border-danger/30 bg-danger-soft p-3">
              <div class="flex items-start gap-2.5">
                <ExclamationCircleIcon class="w-4 h-4 text-danger mt-0.5 shrink-0" />
                <div>
                  <p class="text-xs font-semibold text-danger">Destructive operation</p>
                  <p class="text-[11px] text-danger/80 mt-0.5 leading-relaxed">
                    Restoring will replace all current data with the contents of this backup.
                    Make sure you have verified the backup is valid before proceeding.
                  </p>
                </div>
              </div>
            </div>

            <!-- Backup summary -->
            <div class="flex items-center gap-2.5 p-2.5 rounded-lg bg-surface-hover">
              <ServerStackIcon class="w-4 h-4 text-muted shrink-0" />
              <div class="min-w-0">
                <p class="text-xs font-mono text-foreground truncate">ID: {{ restoreTarget.id }}</p>
                <p class="text-[11px] text-muted font-mono">
                  {{ restoreTarget.type === 'full' ? 'Full backup' : 'Metadata only' }}
                  · {{ formatDate(restoreTarget.created_at) }}
                </p>
              </div>
            </div>

            <!-- Confirmation input: type the backup ID -->
            <div>
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Type
                <code class="font-mono text-danger bg-danger-soft px-1 py-0.5 rounded mx-1 text-[11px]">
                  {{ restoreTarget.id.slice(0, 8) }}
                </code>
                to confirm
              </label>
              <input
                v-model="restoreConfirmInput"
                type="text"
                :placeholder="restoreTarget.id.slice(0, 8)"
                class="w-full px-3 py-2 rounded-lg border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 transition-colors"
                :class="restoreConfirmInput && !restoreConfirmMatch
                  ? 'border-danger focus:ring-danger'
                  : 'border-border focus:ring-danger'"
              />
            </div>

            <div class="flex items-center gap-2 justify-end pt-1">
              <button
                type="button"
                class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                @click="cancelRestore"
              >
                Cancel
              </button>
              <button
                type="button"
                :disabled="!restoreConfirmMatch || restoring"
                class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-danger text-white hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
                @click="confirmRestore"
              >
                <ArrowPathIcon v-if="restoring" class="w-4 h-4 animate-spin" />
                <ArrowPathIcon v-else class="w-4 h-4" />
                Restore now
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { request } from '../../api/client'
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import {
  PlusIcon,
  ArrowPathIcon,
  ArrowDownTrayIcon,
  ArchiveBoxIcon,
  ArchiveBoxArrowDownIcon,
  ServerStackIcon,
  CircleStackIcon,
  ExclamationCircleIcon,
  XMarkIcon,
  ChevronRightIcon,
} from '@heroicons/vue/24/outline'

// ─── Types ─────────────────────────────────────────────────────────────────────

interface BackupJob {
  id: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  type: 'full' | 'metadata-only'
  size_bytes?: number
  created_at: string
  completed_at?: string
  error?: string
}

// ─── Constants ─────────────────────────────────────────────────────────────────

const BACKUP_TYPES = [
  {
    value: 'full',
    label: 'Full backup',
    desc: 'All data: metadata, configuration and artifact binaries.',
    icon: ServerStackIcon,
  },
  {
    value: 'metadata-only',
    label: 'Metadata only',
    desc: 'Repository metadata and configuration, no artifact files.',
    icon: CircleStackIcon,
  },
] as const

// ─── State ─────────────────────────────────────────────────────────────────────

const toasts = useToastStore()

const items = ref<BackupJob[]>([])
const loading = ref(true)
const polling = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

// Create modal
const createModalOpen = ref(false)
const creating = ref(false)
const createForm = ref<{ type: 'full' | 'metadata-only' }>({ type: 'full' })

// Restore modal
const restoreTarget = ref<BackupJob | null>(null)
const restoreConfirmInput = ref('')
const restoring = ref(false)

// ─── Computed ──────────────────────────────────────────────────────────────────

const lastBackup = computed<BackupJob | undefined>(() =>
  [...items.value]
    .filter((j) => j.status === 'completed')
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0],
)

const statTiles = computed(() => [
  {
    label: 'Total backups',
    value: String(items.value.length),
    icon: ArchiveBoxIcon,
    iconBg: 'rgb(var(--color-primary) / 0.1)',
    iconColor: 'rgb(var(--color-primary))',
  },
  {
    label: 'Last backup',
    value: lastBackup.value ? formatDate(lastBackup.value.created_at) : '—',
    icon: ServerStackIcon,
    iconBg: 'rgb(var(--color-success-soft))',
    iconColor: 'rgb(var(--color-success))',
  },
  {
    label: 'Last size',
    value: lastBackup.value?.size_bytes ? formatBytes(lastBackup.value.size_bytes) : '—',
    icon: CircleStackIcon,
    iconBg: 'rgb(var(--color-warning-soft))',
    iconColor: 'rgb(var(--color-warning))',
  },
])

const restoreConfirmMatch = computed(
  () =>
    restoreTarget.value !== null &&
    restoreConfirmInput.value === restoreTarget.value.id.slice(0, 8),
)

const hasActiveJobs = computed(() =>
  items.value.some((j) => j.status === 'pending' || j.status === 'running'),
)

// ─── Helpers ───────────────────────────────────────────────────────────────────

function statusLabel(s: string): string {
  const map: Record<string, string> = {
    pending: 'Pending',
    running: 'Running',
    completed: 'Completed',
    failed: 'Failed',
  }
  return map[s] ?? s
}

function statusBadgeClass(s: string): string {
  const map: Record<string, string> = {
    pending: 'bg-surface-hover text-muted',
    running: 'bg-warning-soft text-warning',
    completed: 'bg-success-soft text-success',
    failed: 'bg-danger-soft text-danger',
  }
  return map[s] ?? 'bg-surface-hover text-muted'
}

function statusDotClass(s: string): string {
  const map: Record<string, string> = {
    pending: 'bg-muted',
    running: 'bg-warning',
    completed: 'bg-success',
    failed: 'bg-danger',
  }
  return map[s] ?? 'bg-muted'
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  } catch {
    return iso
  }
}

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

// ─── Data Loading ──────────────────────────────────────────────────────────────

async function load(): Promise<void> {
  try {
    const res = await request<{ items: BackupJob[] }>('/api/v1/admin/backup')
    items.value = (res?.items ?? []).sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    )
  } finally {
    loading.value = false
  }

  // Manage auto-refresh
  if (hasActiveJobs.value) {
    startPolling()
  } else {
    stopPolling()
  }
}

function startPolling(): void {
  if (pollTimer) return
  polling.value = true
  pollTimer = setInterval(async () => {
    await load()
    if (!hasActiveJobs.value) stopPolling()
  }, 5000)
}

function stopPolling(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  polling.value = false
}

// ─── Create Backup ─────────────────────────────────────────────────────────────

function openCreateModal(): void {
  createForm.value = { type: 'full' }
  createModalOpen.value = true
}

function closeCreateModal(): void {
  createModalOpen.value = false
}

async function confirmCreate(): Promise<void> {
  creating.value = true
  try {
    await request('/api/v1/admin/backup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: createForm.value.type }),
    })
    toasts.success('Backup job started')
    closeCreateModal()
    loading.value = false
    await load()
    startPolling()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to create backup')
  } finally {
    creating.value = false
  }
}

// ─── Restore ───────────────────────────────────────────────────────────────────

function openRestoreModal(job: BackupJob): void {
  restoreTarget.value = job
  restoreConfirmInput.value = ''
}

function cancelRestore(): void {
  restoreTarget.value = null
  restoreConfirmInput.value = ''
}

async function confirmRestore(): Promise<void> {
  if (!restoreTarget.value || !restoreConfirmMatch.value) return
  restoring.value = true
  try {
    await request('/api/v1/admin/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ backup_id: restoreTarget.value.id }),
    })
    toasts.success('Restore job started')
    cancelRestore()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to start restore')
  } finally {
    restoring.value = false
  }
}

// ─── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(load)
onUnmounted(stopPolling)
</script>
