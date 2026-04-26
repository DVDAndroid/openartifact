<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-5">

    <!-- ─── Header row ──────────────────────────────────────────────────────── -->
    <div class="flex items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-foreground">Cleanup Policies</h2>
        <p class="text-sm text-muted mt-0.5">Automatically remove old artifacts based on retention rules.</p>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity shrink-0"
        @click="openDrawer('create')"
      >
        <PlusIcon class="w-4 h-4" />
        Add policy
      </button>
    </div>

    <!-- ─── Loading skeleton ────────────────────────────────────────────────── -->
    <div v-if="loading" class="bg-surface rounded-xl border border-border overflow-hidden">
      <div v-for="i in 3" :key="i" class="flex items-center gap-4 px-5 py-4 border-b border-border last:border-0">
        <div class="h-4 w-36 rounded skel" />
        <div class="h-5 w-24 rounded skel" />
        <div class="h-4 w-28 rounded skel" />
        <div class="flex-1" />
        <div class="w-8 h-5 rounded-full skel" />
        <div class="flex gap-1">
          <div class="w-7 h-7 rounded-md skel" />
          <div class="w-7 h-7 rounded-md skel" />
          <div class="w-7 h-7 rounded-md skel" />
        </div>
      </div>
    </div>

    <!-- ─── Table ───────────────────────────────────────────────────────────── -->
    <div v-else class="bg-surface rounded-xl border border-border overflow-hidden">
      <!-- Empty state -->
      <div v-if="items.length === 0" class="py-16 text-center">
        <div class="w-12 h-12 rounded-full bg-surface-hover mx-auto flex items-center justify-center mb-3">
          <TrashIcon class="w-6 h-6 text-muted" />
        </div>
        <p class="text-sm font-medium text-foreground">No cleanup policies</p>
        <p class="text-xs text-muted mt-1 mb-4">Add a policy to automatically prune old artifact versions.</p>
        <button
          type="button"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
          @click="openDrawer('create')"
        >
          <PlusIcon class="w-4 h-4" />
          Add policy
        </button>
      </div>

      <!-- Data table -->
      <table v-else class="w-full">
        <thead>
          <tr class="bg-surface-hover/30">
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5">
              Name
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Type
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Rule
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Enabled
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Last run
            </th>
            <th class="text-right text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-border">
          <tr
            v-for="policy in items"
            :key="policy.id"
            class="hover:bg-surface-hover/30 transition-colors"
          >
            <!-- Name -->
            <td class="px-5 py-3.5">
              <span class="font-medium text-sm text-foreground">{{ policy.name }}</span>
            </td>

            <!-- Type chip -->
            <td class="px-3 py-3.5">
              <span
                class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border border-border bg-surface-hover text-[11px] font-mono font-medium"
                :class="policy.type === 'keep-last-n' ? 'text-primary' : 'text-warning'"
              >
                <component :is="policy.type === 'keep-last-n' ? QueueListIcon : CalendarIcon" class="w-3.5 h-3.5" />
                {{ policy.type === 'keep-last-n' ? 'keep-last-n' : 'keep-by-age' }}
              </span>
            </td>

            <!-- Rule description -->
            <td class="px-3 py-3.5 text-sm text-muted">
              <span v-if="policy.type === 'keep-last-n'">
                Keep last <span class="font-mono font-semibold text-foreground">{{ policy.keep_count ?? '?' }}</span>
              </span>
              <span v-else>
                Keep <span class="font-mono font-semibold text-foreground">{{ policy.keep_days ?? '?' }}</span> days
              </span>
            </td>

            <!-- Enabled toggle -->
            <td class="px-3 py-3.5">
              <button
                type="button"
                role="switch"
                :aria-checked="policy.enabled"
                :disabled="togglingId === policy.id"
                class="relative inline-flex h-5 w-9 shrink-0 rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50"
                :class="policy.enabled ? 'bg-primary' : 'bg-border'"
                @click="toggleEnabled(policy)"
              >
                <span
                  class="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform duration-200 mt-0.5"
                  :class="policy.enabled ? 'translate-x-4' : 'translate-x-0.5'"
                />
              </button>
            </td>

            <!-- Last run -->
            <td class="px-3 py-3.5">
              <span v-if="policy.last_run_at" class="text-sm text-muted font-mono">
                {{ timeAgo(policy.last_run_at) }}
              </span>
              <span v-else class="text-sm text-muted/50">Never</span>
            </td>

            <!-- Actions -->
            <td class="px-5 py-3.5">
              <div class="flex items-center gap-0.5 justify-end">
                <!-- Run now -->
                <button
                  type="button"
                  title="Run now"
                  :disabled="runningId === policy.id"
                  class="p-1.5 rounded-md text-muted hover:text-success hover:bg-success/10 transition-colors disabled:opacity-40"
                  @click="runPolicy(policy)"
                >
                  <ArrowPathIcon v-if="runningId === policy.id" class="w-4 h-4 animate-spin" />
                  <PlayIcon v-else class="w-4 h-4" />
                </button>
                <!-- Edit -->
                <button
                  type="button"
                  title="Edit policy"
                  class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  @click="openDrawer('edit', policy)"
                >
                  <PencilIcon class="w-4 h-4" />
                </button>
                <!-- Delete -->
                <button
                  type="button"
                  title="Delete policy"
                  class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                  @click="openDeleteModal(policy)"
                >
                  <TrashIcon class="w-4 h-4" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ═══════════════════════════════════════════════════════════════════════
         SLIDE-RIGHT DRAWER
    ════════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div v-if="drawerOpen" class="fixed inset-0 z-50 flex justify-end">
        <!-- Backdrop -->
        <div
          class="absolute inset-0 bg-black/40 backdrop-blur-sm animate-fade-in"
          @click="closeDrawer"
        />

        <!-- Drawer panel -->
        <div class="relative w-full max-w-md bg-surface border-l border-border shadow-2xl flex flex-col animate-slideRight">
          <!-- Header -->
          <div class="px-6 py-5 border-b border-border shrink-0">
            <p class="text-[10px] uppercase tracking-[0.14em] font-mono text-muted mb-0.5">
              {{ drawerMode === 'create' ? 'New policy' : 'Edit policy' }}
            </p>
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-bold text-foreground tracking-tight">
                {{ drawerMode === 'create' ? 'Add Cleanup Policy' : `Edit · ${editingPolicy?.name}` }}
              </h2>
              <button
                type="button"
                class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors shrink-0"
                @click="closeDrawer"
              >
                <XMarkIcon class="w-5 h-5" />
              </button>
            </div>
          </div>

          <!-- Body -->
          <div class="flex-1 overflow-y-auto px-6 py-5 space-y-5">
            <!-- Name -->
            <div>
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Policy name <span class="text-danger ml-0.5">*</span>
              </label>
              <input
                v-model="form.name"
                type="text"
                placeholder="e.g. prune-old-releases"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
              />
            </div>

            <!-- Type selector -->
            <div>
              <label class="block text-xs font-semibold text-foreground mb-2">
                Policy type <span class="text-danger ml-0.5">*</span>
              </label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="opt in POLICY_TYPES"
                  :key="opt.value"
                  type="button"
                  class="relative text-left p-3 rounded-lg border overflow-hidden transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  :class="
                    form.type === opt.value
                      ? 'border-primary bg-primary/5 ring-1 ring-primary/30'
                      : 'border-border bg-surface hover:bg-surface-hover cursor-pointer'
                  "
                  @click="form.type = opt.value as 'keep-last-n' | 'keep-by-age'"
                >
                  <span
                    class="absolute left-0 top-0 bottom-0 w-[3px] rounded-l-lg"
                    :style="{ background: opt.color }"
                  />
                  <div class="pl-3">
                    <component :is="opt.icon" class="w-4 h-4 mb-1" :style="{ color: opt.color }" />
                    <p class="text-[12px] font-semibold text-foreground leading-tight">{{ opt.label }}</p>
                    <p class="text-[11px] text-muted leading-snug mt-0.5">{{ opt.desc }}</p>
                  </div>
                </button>
              </div>
            </div>

            <!-- Conditional rule input -->
            <div v-if="form.type === 'keep-last-n'">
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Keep count <span class="text-danger ml-0.5">*</span>
              </label>
              <div class="flex items-center gap-2">
                <input
                  v-model.number="form.keep_count"
                  type="number"
                  min="1"
                  max="9999"
                  placeholder="10"
                  class="w-28 px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                />
                <span class="text-sm text-muted">versions per package</span>
              </div>
            </div>

            <div v-else-if="form.type === 'keep-by-age'">
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Retention period <span class="text-danger ml-0.5">*</span>
              </label>
              <div class="flex items-center gap-2">
                <input
                  v-model.number="form.keep_days"
                  type="number"
                  min="1"
                  max="3650"
                  placeholder="30"
                  class="w-28 px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                />
                <span class="text-sm text-muted">days</span>
              </div>
            </div>

            <!-- Enabled toggle -->
            <div class="flex items-start gap-3 p-3.5 rounded-lg border border-border bg-surface-hover/30">
              <button
                type="button"
                role="switch"
                :aria-checked="form.enabled"
                class="relative inline-flex h-5 w-9 shrink-0 rounded-full transition-colors duration-200 mt-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                :class="form.enabled ? 'bg-primary' : 'bg-border'"
                @click="form.enabled = !form.enabled"
              >
                <span
                  class="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform duration-200 mt-0.5"
                  :class="form.enabled ? 'translate-x-4' : 'translate-x-0.5'"
                />
              </button>
              <div>
                <p class="text-xs font-semibold text-foreground">Enable policy</p>
                <p class="text-[11px] text-muted mt-0.5 leading-relaxed">
                  Disabled policies won't run automatically or in scheduled jobs.
                </p>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-border flex items-center gap-2 shrink-0 bg-surface">
            <div class="flex-1" />
            <button
              type="button"
              class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
              @click="closeDrawer"
            >
              Cancel
            </button>
            <button
              type="button"
              :disabled="savingDrawer || !formValid"
              class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
              @click="saveDrawer"
            >
              <ArrowPathIcon v-if="savingDrawer" class="w-4 h-4 animate-spin" />
              {{ drawerMode === 'create' ? 'Create policy' : 'Save changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ═══════════════════════════════════════════════════════════════════════
         DELETE CONFIRMATION MODAL
    ════════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="deleteTarget"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in"
      >
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="cancelDelete" />
        <div class="relative bg-surface border border-border rounded-xl shadow-2xl w-full max-w-sm">
          <div class="px-6 pt-6 pb-4">
            <h2 class="text-base font-bold text-foreground tracking-tight">Delete policy</h2>
            <p class="text-sm text-muted mt-0.5">
              Delete
              <span class="font-mono font-semibold text-foreground">{{ deleteTarget.name }}</span>?
              This action cannot be undone.
            </p>
          </div>
          <div class="px-6 pb-6 flex items-center gap-2 justify-end">
            <button
              type="button"
              class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
              @click="cancelDelete"
            >
              Cancel
            </button>
            <button
              type="button"
              :disabled="deleting"
              class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-danger text-white hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
              @click="confirmDelete"
            >
              <ArrowPathIcon v-if="deleting" class="w-4 h-4 animate-spin" />
              <TrashIcon v-else class="w-4 h-4" />
              Delete
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { request } from '../api/client'
import { useToastStore } from '../stores/toast'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  ArrowPathIcon,
  PlayIcon,
  XMarkIcon,
  QueueListIcon,
  CalendarIcon,
} from '@heroicons/vue/24/outline'

// ─── Props ─────────────────────────────────────────────────────────────────────

const props = defineProps<{
  repoName: string
}>()

// ─── Types ─────────────────────────────────────────────────────────────────────

interface CleanupPolicy {
  id: string
  name: string
  type: 'keep-last-n' | 'keep-by-age'
  keep_count?: number
  keep_days?: number
  enabled: boolean
  last_run_at?: string
  created_at: string
}

// ─── Constants ─────────────────────────────────────────────────────────────────

const POLICY_TYPES = [
  {
    value: 'keep-last-n',
    label: 'Keep last N',
    desc: 'Retain only the N most recent versions.',
    icon: QueueListIcon,
    color: '#6366f1',
  },
  {
    value: 'keep-by-age',
    label: 'Keep by age',
    desc: 'Remove versions older than N days.',
    icon: CalendarIcon,
    color: '#f59e0b',
  },
] as const

// ─── State ─────────────────────────────────────────────────────────────────────

const toasts = useToastStore()

const items = ref<CleanupPolicy[]>([])
const loading = ref(true)

// Running
const runningId = ref<string | null>(null)
const togglingId = ref<string | null>(null)

// Drawer
const drawerOpen = ref(false)
const drawerMode = ref<'create' | 'edit'>('create')
const editingPolicy = ref<CleanupPolicy | null>(null)
const savingDrawer = ref(false)

const form = reactive<{
  name: string
  type: 'keep-last-n' | 'keep-by-age'
  keep_count: number | undefined
  keep_days: number | undefined
  enabled: boolean
}>({
  name: '',
  type: 'keep-last-n',
  keep_count: 10,
  keep_days: 30,
  enabled: true,
})

// Delete
const deleteTarget = ref<CleanupPolicy | null>(null)
const deleting = ref(false)

// ─── Computed ──────────────────────────────────────────────────────────────────

const formValid = computed(() => {
  if (!form.name.trim()) return false
  if (form.type === 'keep-last-n') return (form.keep_count ?? 0) > 0
  if (form.type === 'keep-by-age') return (form.keep_days ?? 0) > 0
  return false
})

// ─── Base URL helper ───────────────────────────────────────────────────────────

function baseUrl(): string {
  return `/api/v1/repositories/${props.repoName}/cleanup-policies`
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

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
  loading.value = true
  try {
    const res = await request<{ items: CleanupPolicy[] }>(baseUrl())
    items.value = res?.items ?? []
  } finally {
    loading.value = false
  }
}

// ─── Toggle Enabled ────────────────────────────────────────────────────────────

async function toggleEnabled(policy: CleanupPolicy): Promise<void> {
  togglingId.value = policy.id
  try {
    await request(`${baseUrl()}/${policy.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...policy, enabled: !policy.enabled }),
    })
    policy.enabled = !policy.enabled
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to update policy')
  } finally {
    togglingId.value = null
  }
}

// ─── Run Now ───────────────────────────────────────────────────────────────────

async function runPolicy(policy: CleanupPolicy): Promise<void> {
  runningId.value = policy.id
  try {
    await request(`${baseUrl()}/${policy.id}/run`, { method: 'POST' })
    toasts.success(`Policy "${policy.name}" ran successfully`)
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to run policy')
  } finally {
    runningId.value = null
  }
}

// ─── Drawer Actions ────────────────────────────────────────────────────────────

function resetForm(): void {
  form.name = ''
  form.type = 'keep-last-n'
  form.keep_count = 10
  form.keep_days = 30
  form.enabled = true
}

function openDrawer(mode: 'create' | 'edit', policy?: CleanupPolicy): void {
  drawerMode.value = mode
  if (mode === 'edit' && policy) {
    editingPolicy.value = policy
    form.name = policy.name
    form.type = policy.type
    form.keep_count = policy.keep_count
    form.keep_days = policy.keep_days
    form.enabled = policy.enabled
  } else {
    resetForm()
    editingPolicy.value = null
  }
  drawerOpen.value = true
}

function closeDrawer(): void {
  drawerOpen.value = false
  editingPolicy.value = null
}

async function saveDrawer(): Promise<void> {
  if (!formValid.value) return
  savingDrawer.value = true
  try {
    const payload: Record<string, unknown> = {
      name: form.name,
      type: form.type,
      enabled: form.enabled,
    }
    if (form.type === 'keep-last-n') payload.keep_count = form.keep_count
    else payload.keep_days = form.keep_days

    if (drawerMode.value === 'create') {
      await request(baseUrl(), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      toasts.success(`Policy "${form.name}" created`)
    } else if (editingPolicy.value) {
      await request(`${baseUrl()}/${editingPolicy.value.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      toasts.success(`Policy "${form.name}" updated`)
    }

    closeDrawer()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Save failed')
  } finally {
    savingDrawer.value = false
  }
}

// ─── Delete Actions ────────────────────────────────────────────────────────────

function openDeleteModal(policy: CleanupPolicy): void {
  deleteTarget.value = policy
}

function cancelDelete(): void {
  deleteTarget.value = null
}

async function confirmDelete(): Promise<void> {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await request(`${baseUrl()}/${deleteTarget.value.id}`, { method: 'DELETE' })
    toasts.success(`Policy "${deleteTarget.value.name}" deleted`)
    deleteTarget.value = null
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Delete failed')
  } finally {
    deleting.value = false
  }
}

// ─── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(load)
</script>
