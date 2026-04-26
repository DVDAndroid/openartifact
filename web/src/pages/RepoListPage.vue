<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="p-6">
    <!-- Header Row -->
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold tracking-tight">Repositories</h1>
      <div class="flex items-center gap-2">
        <!-- Search -->
        <div class="relative">
          <MagnifyingGlassIcon class="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted pointer-events-none" />
          <input
            v-model="searchQuery"
            type="search"
            placeholder="Search repositories…"
            class="h-8 w-52 pl-7 pr-2 rounded-md text-xs bg-surface-hover/60 border border-border text-foreground placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
        <button
          @click="showNewModal = true"
          class="bg-primary text-primary-contrast rounded-lg px-4 py-2 text-sm font-medium hover:bg-primary/90 active:scale-[0.97] transition inline-flex items-center gap-1.5"
        >
          New Repository
        </button>
      </div>
    </div>

    <!-- Format Filter Pills + View Toggle -->
    <div class="flex items-center justify-between gap-4 mt-4">
      <!-- Format Pills -->
      <div class="flex flex-wrap gap-2">
        <!-- All pill -->
        <button
          @click="activeFormat = null"
          :class="[
            'px-3 py-1.5 rounded-full text-xs font-medium border transition-colors cursor-pointer inline-flex items-center gap-1.5',
            activeFormat === null
              ? 'border-primary/40 bg-primary-soft text-primary'
              : 'border-border bg-surface hover:bg-surface-hover text-foreground/75'
          ]"
        >
          <div class="w-1.5 h-1.5 rounded-full bg-muted" />
          All
          <span class="text-[10px] font-mono tabular-nums">{{ repos.length }}</span>
        </button>

        <!-- Format pills -->
        <button
          v-for="[format, count] of formatCounts"
          :key="format"
          @click="activeFormat = format"
          :class="[
            'px-3 py-1.5 rounded-full text-xs font-medium border transition-colors cursor-pointer inline-flex items-center gap-1.5',
            activeFormat === format
              ? 'border-primary/40 bg-primary-soft text-primary'
              : 'border-border bg-surface hover:bg-surface-hover text-foreground/75'
          ]"
        >
          <div
            class="w-1.5 h-1.5 rounded-full"
            :style="{ backgroundColor: formatColor(format) }"
          />
          {{ format }}
          <span class="text-[10px] font-mono tabular-nums">{{ count }}</span>
        </button>
      </div>

      <!-- View Toggle -->
      <div class="flex items-center gap-1 bg-surface-hover rounded-lg p-0.5">
        <button
          @click="view = 'grid'"
          :class="[
            'p-1.5 rounded-md transition-colors',
            view === 'grid'
              ? 'bg-surface text-foreground shadow-sm'
              : 'text-muted hover:text-foreground'
          ]"
          title="Grid view"
        >
          <Squares2X2Icon class="w-4 h-4" />
        </button>
        <button
          @click="view = 'list'"
          :class="[
            'p-1.5 rounded-md transition-colors',
            view === 'list'
              ? 'bg-surface text-foreground shadow-sm'
              : 'text-muted hover:text-foreground'
          ]"
          title="List view"
        >
          <Bars3Icon class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Loading State -->
    <template v-if="loading">
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mt-4">
        <div v-for="i in 6" :key="i" class="rounded-xl border border-border bg-surface p-5 animate-pulse">
          <div class="flex items-center gap-2 mb-3">
            <div class="h-4 w-32 bg-surface-hover rounded" />
            <div class="h-5 w-14 bg-surface-hover rounded" />
          </div>
          <div class="flex gap-1.5 mb-4">
            <div class="h-4 w-14 bg-surface-hover rounded" />
            <div class="h-4 w-16 bg-surface-hover rounded" />
          </div>
          <div class="h-3 w-full bg-surface-hover rounded mb-1.5" />
          <div class="h-3 w-2/3 bg-surface-hover rounded mb-4" />
          <div class="flex items-center justify-between">
            <div class="h-7 w-24 bg-surface-hover rounded" />
            <div class="h-3 w-16 bg-surface-hover rounded" />
          </div>
          <div class="mt-3 h-5 w-full bg-surface-hover rounded" />
        </div>
      </div>
    </template>

    <!-- Empty State -->
    <template v-else-if="repos.length === 0">
      <EmptyState title="No repositories" description="Create your first repository to get started" />
    </template>

    <!-- Grid View -->
    <template v-else-if="view === 'grid'">
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mt-4">
        <div
          v-for="repo of filteredRepos"
          :key="repo.id"
          class="group relative text-left rounded-xl border bg-surface p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/5 dark:hover:shadow-black/40"
          :class="selectedIds.has(repo.name)
            ? 'border-primary bg-primary-soft/20'
            : 'border-border hover:border-primary/30'"
        >
          <!-- Left accent rail -->
          <div
            class="absolute left-0 top-4 bottom-4 w-[2px] rounded-r"
            :style="{ background: `linear-gradient(180deg, ${formatColor(repo.format)}, transparent 80%)` }"
          />

          <!-- Top row: checkbox + name + format badge -->
          <div class="flex items-start justify-between gap-3 mb-3">
            <div class="flex items-center gap-2 flex-1 min-w-0">
              <input
                type="checkbox"
                :checked="selectedIds.has(repo.name)"
                class="w-4 h-4 rounded border-border text-primary focus:ring-primary cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                :class="{ 'opacity-100': selectedIds.has(repo.name) }"
                @click.stop
                @change="toggleRepo(repo.name)"
              />
              <h3
                class="font-semibold tracking-tight flex-1 truncate cursor-pointer hover:text-primary transition-colors"
                @click="router.push(`/repos/${repo.name}`)"
              >{{ repo.name }}</h3>
            </div>
            <FormatBadge :format="repo.format" />
          </div>

          <!-- Chips row: type + visibility -->
          <div class="flex flex-wrap gap-2 mb-3">
            <span class="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-md bg-surface-hover text-muted">
              {{ (repo as any).type || 'hosted' }}
            </span>
            <span class="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-md bg-surface-hover text-muted">
              <EyeIcon v-if="repo.public" class="w-3 h-3" />
              <EyeSlashIcon v-else class="w-3 h-3" />
              {{ repo.public ? 'Public' : 'Private' }}
            </span>
          </div>

          <!-- Description -->
          <p class="text-sm text-muted leading-relaxed line-clamp-2 mb-4">
            {{ repo.description || 'No description provided' }}
          </p>

          <!-- Bottom row: stats + last push -->
          <div class="flex items-center justify-between text-xs text-muted">
            <div class="flex items-center gap-3">
              <span class="font-mono tabular-nums">—</span>
              <span class="font-mono tabular-nums">—</span>
            </div>
            <div class="inline-flex items-center gap-1">
              <ClockIcon class="w-3 h-3" />
              <span>—</span>
            </div>
          </div>

          <!-- Sparkline -->
          <div class="mt-3">
            <Sparkline :data="fakeSparkline()" />
          </div>
        </div>
      </div>
    </template>

    <!-- List View -->
    <template v-else>
      <div class="bg-surface rounded-xl border border-border overflow-hidden mt-4">
        <!-- Table Header -->
        <div class="grid grid-cols-[28px_2fr_1fr_1fr_1fr_1fr_1fr_1fr] gap-4 bg-surface-hover/50 px-5 py-3 text-xs text-muted uppercase tracking-wider border-b border-border">
          <div>
            <input
              type="checkbox"
              :checked="allSelected"
              :indeterminate="someSelected && !allSelected"
              class="w-4 h-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
              @change="toggleAll"
            />
          </div>
          <div>Name</div>
          <div>Format</div>
          <div>Type</div>
          <div class="text-right">Artifacts</div>
          <div class="text-right">Storage</div>
          <div class="text-right">Last push</div>
          <div>Activity</div>
        </div>

        <!-- Table Rows -->
        <div
          v-for="repo of filteredRepos"
          :key="repo.id"
          class="grid grid-cols-[28px_2fr_1fr_1fr_1fr_1fr_1fr_1fr] gap-4 px-5 py-4 border-t border-border hover:bg-surface-hover transition-colors cursor-pointer items-center group"
          :class="{ 'bg-primary-soft/20': selectedIds.has(repo.name) }"
          @click="router.push(`/repos/${repo.name}`)"
        >
          <!-- Checkbox -->
          <div @click.stop>
            <input
              type="checkbox"
              :checked="selectedIds.has(repo.name)"
              class="w-4 h-4 rounded border-border text-primary focus:ring-primary cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity"
              :class="{ 'opacity-100': selectedIds.has(repo.name) }"
              @change="toggleRepo(repo.name)"
            />
          </div>

          <!-- Name -->
          <div class="font-medium text-foreground truncate">{{ repo.name }}</div>

          <!-- Format -->
          <div>
            <FormatBadge :format="repo.format" />
          </div>

          <!-- Type -->
          <div>
            <span class="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-md bg-surface-hover text-muted">
              {{ (repo as any).type || 'hosted' }}
            </span>
          </div>

          <!-- Artifacts -->
          <div class="text-right text-xs text-muted font-mono tabular-nums">—</div>

          <!-- Storage -->
          <div class="text-right text-xs text-muted font-mono tabular-nums">—</div>

          <!-- Last push -->
          <div class="text-right text-xs text-muted">—</div>

          <!-- Sparkline -->
          <div>
            <Sparkline :data="fakeSparkline()" />
          </div>
        </div>
      </div>
    </template>

    <!-- New Repository Modal -->
    <NewRepoModal v-model="showNewModal" @update:modelValue="!$event && fetchRepos()" />

    <!-- Bulk Actions Bar -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-all duration-300 ease-out"
        enter-from-class="translate-y-full opacity-0"
        enter-to-class="translate-y-0 opacity-100"
        leave-active-class="transition-all duration-200 ease-in"
        leave-from-class="translate-y-0 opacity-100"
        leave-to-class="translate-y-full opacity-0"
      >
        <div
          v-if="selectedIds.size > 0"
          class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 w-full max-w-2xl px-4"
        >
          <div class="flex items-center gap-2 px-4 py-3 rounded-xl border border-border bg-surface/95 backdrop-blur-sm shadow-2xl shadow-black/20 ring-1 ring-black/5">
            <!-- Count badge -->
            <div class="flex items-center gap-2 shrink-0">
              <span class="inline-flex items-center justify-center w-6 h-6 rounded-full bg-primary text-primary-contrast text-xs font-bold">
                {{ selectedIds.size > 99 ? '99+' : selectedIds.size }}
              </span>
              <span class="text-sm font-medium text-foreground whitespace-nowrap">
                {{ selectedIds.size === 1 ? '1 repository selected' : `${selectedIds.size} repositories selected` }}
              </span>
            </div>

            <div class="w-px h-5 bg-border mx-1 shrink-0" />

            <!-- Delete action -->
            <button
              class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md text-danger border border-danger/30 hover:bg-danger-soft transition-colors disabled:opacity-50"
              :disabled="bulkBusy"
              @click="confirmBulkDelete = true"
            >
              <TrashIcon class="w-3.5 h-3.5" />
              Delete selected
            </button>

            <!-- Progress -->
            <div v-if="bulkBusy && bulkProgress > 0" class="flex items-center gap-2 shrink-0">
              <div class="w-20 h-1.5 bg-border rounded-full overflow-hidden">
                <div class="h-full bg-primary rounded-full transition-all duration-300" :style="{ width: `${bulkProgress}%` }" />
              </div>
              <span class="text-xs text-muted tabular-nums">{{ bulkProgress }}%</span>
            </div>

            <!-- Dismiss -->
            <button
              class="shrink-0 w-7 h-7 flex items-center justify-center rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors ml-auto"
              title="Clear selection"
              @click="clearSelection"
            >
              <XMarkIcon class="w-4 h-4" />
            </button>
          </div>
        </div>
      </Transition>

      <!-- Delete confirmation modal -->
      <Transition
        enter-active-class="transition-opacity duration-200"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="confirmBulkDelete"
          class="fixed inset-0 z-50 flex items-center justify-center p-4"
          @click.self="confirmBulkDelete = false"
        >
          <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="confirmBulkDelete = false" />
          <div class="relative w-full max-w-sm rounded-xl border border-border bg-surface shadow-2xl p-6" role="dialog" aria-modal="true">
            <div class="flex items-start gap-4">
              <div class="w-10 h-10 rounded-full bg-danger-soft flex items-center justify-center shrink-0">
                <ExclamationTriangleIcon class="w-5 h-5 text-danger" />
              </div>
              <div class="flex-1 min-w-0">
                <h2 class="text-base font-semibold text-foreground">
                  Delete {{ selectedIds.size }} {{ selectedIds.size === 1 ? 'repository' : 'repositories' }}?
                </h2>
                <p class="mt-1 text-sm text-muted">This action cannot be undone. All artifacts inside will be permanently removed.</p>
              </div>
            </div>
            <div class="flex justify-end gap-2 mt-6">
              <button
                class="px-4 py-2 text-sm rounded-md border border-border text-foreground hover:bg-surface-hover transition-colors"
                @click="confirmBulkDelete = false"
              >Cancel</button>
              <button
                class="flex items-center gap-2 px-4 py-2 text-sm rounded-md bg-danger text-white hover:opacity-90 transition-opacity disabled:opacity-50"
                :disabled="bulkBusy"
                @click="doBulkDelete"
              >
                <ArrowPathIcon v-if="bulkBusy" class="w-3.5 h-3.5 animate-spin" />
                <TrashIcon v-else class="w-3.5 h-3.5" />
                {{ bulkBusy ? 'Deleting…' : 'Delete' }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Squares2X2Icon, Bars3Icon, EyeIcon, EyeSlashIcon, ClockIcon,
  MagnifyingGlassIcon, TrashIcon, XMarkIcon, ExclamationTriangleIcon, ArrowPathIcon,
} from '@heroicons/vue/24/outline'
import { listRepositories, deleteRepository } from '../api/repositories'
import type { Repository } from '../api/repositories'
import FormatBadge from '../components/FormatBadge.vue'
import Sparkline from '../components/Sparkline.vue'
import EmptyState from '../components/EmptyState.vue'
import NewRepoModal from '../components/NewRepoModal.vue'
import { useToastStore } from '../stores/toast'
import { formatColor } from '../lib/formats'

const router = useRouter()
const toasts = useToastStore()
const repos = ref<Repository[]>([])
const loading = ref(true)
const view = ref<'grid' | 'list'>('grid')
const activeFormat = ref<string | null>(null)
const showNewModal = ref(false)
const searchQuery = ref('')

// ── Selection ──────────────────────────────────────────────────────────────
const selectedIds = ref<Set<string>>(new Set())

const allSelected = computed(() =>
  filteredRepos.value.length > 0 && filteredRepos.value.every(r => selectedIds.value.has(r.name))
)
const someSelected = computed(() =>
  filteredRepos.value.some(r => selectedIds.value.has(r.name))
)

function toggleRepo(name: string): void {
  const next = new Set(selectedIds.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  selectedIds.value = next
}

function toggleAll(): void {
  if (allSelected.value) {
    const next = new Set(selectedIds.value)
    filteredRepos.value.forEach(r => next.delete(r.name))
    selectedIds.value = next
  } else {
    const next = new Set(selectedIds.value)
    filteredRepos.value.forEach(r => next.add(r.name))
    selectedIds.value = next
  }
}

function clearSelection(): void {
  selectedIds.value = new Set()
}

// ── Bulk delete ────────────────────────────────────────────────────────────
const confirmBulkDelete = ref(false)
const bulkBusy = ref(false)
const bulkProgress = ref(0)

async function doBulkDelete(): Promise<void> {
  bulkBusy.value = true
  bulkProgress.value = 0
  const toDelete = [...selectedIds.value]
  let succeeded = 0

  for (let i = 0; i < toDelete.length; i++) {
    try {
      await deleteRepository(toDelete[i])
      succeeded++
    } catch { /* continue */ }
    bulkProgress.value = Math.round(((i + 1) / toDelete.length) * 100)
  }

  repos.value = repos.value.filter(r => !selectedIds.value.has(r.name) || !toDelete.includes(r.name))
  // Refresh to get accurate state
  await fetchRepos()
  clearSelection()
  confirmBulkDelete.value = false
  bulkBusy.value = false
  bulkProgress.value = 0

  if (succeeded > 0) toasts.success(`Deleted ${succeeded} ${succeeded === 1 ? 'repository' : 'repositories'}`)
  if (succeeded < toDelete.length) toasts.error('Some repositories could not be deleted')
}

function fakeSparkline() {
  return Array.from({ length: 8 }, () => Math.random() * 100)
}

const formatCounts = computed(() => {
  const counts = new Map<string, number>()
  repos.value.forEach(r => counts.set(r.format, (counts.get(r.format) || 0) + 1))
  return counts
})

const filteredRepos = computed(() => {
  let result = repos.value
  if (activeFormat.value) result = result.filter(r => r.format === activeFormat.value)
  const q = searchQuery.value.trim().toLowerCase()
  if (q) result = result.filter(r => r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q))
  return result
})

async function fetchRepos() {
  loading.value = true
  try {
    const res = await listRepositories({ limit: 100 })
    repos.value = res.repositories || []
  } finally {
    loading.value = false
  }
}

onMounted(fetchRepos)
</script>
