<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div>
    <!-- Loading State -->
    <div v-if="loading" class="text-muted">Loading...</div>

    <!-- Main Content -->
    <template v-else-if="repo">
      <div class="rounded-xl border border-border bg-surface overflow-hidden">
        <!-- Hero Section -->
        <div class="relative px-6 py-5 border-b border-border">
          <!-- Radial gradient overlay -->
          <div
            class="absolute inset-0 opacity-[0.06] pointer-events-none"
            :style="{ background: `radial-gradient(60% 40% at 0% 0%, ${repoFormatColor}, transparent 70%)` }"
          />

          <div class="relative">
            <!-- Title row -->
            <div class="flex items-center gap-2.5 flex-wrap">
              <h1 class="text-2xl font-bold tracking-tight text-foreground">{{ repo.name }}</h1>
              <FormatBadge :format="repo.format" />
              <span
                class="text-[10px] font-medium px-2 py-0.5 rounded-full border"
                :class="repo.public
                  ? 'border-green-500/30 text-green-600 dark:text-green-400 bg-green-500/10'
                  : 'border-border text-muted bg-surface-hover'
                "
              >
                {{ repo.public ? 'Public' : 'Private' }}
              </span>
              <span class="text-[10px] font-medium px-2 py-0.5 rounded-full border border-border text-muted bg-surface-hover">
                {{ repo.format }}
              </span>
            </div>

            <!-- Description -->
            <p v-if="repo.description" class="text-sm text-muted max-w-2xl leading-relaxed mt-2">
              {{ repo.description }}
            </p>

            <!-- Stats row -->
            <div class="flex items-center gap-5 flex-wrap mt-3.5">
              <HeroStat label="Artifacts" :value="String(totalPackages)" />
              <HeroStat label="Format" :value="repo.format" />
              <HeroStat label="Last push" value="—" />
              <HeroStat label="Created" value="—" />
              <HeroStat label="URL" :value="`/${repo.name}/`" mono />
            </div>

            <!-- Action button group -->
            <div class="flex items-center justify-between gap-3 mt-4 flex-wrap">
              <div class="inline-flex items-center rounded-lg border border-border bg-surface overflow-hidden shrink-0 shadow-sm">
                <router-link
                  :to="`/repos/${repo.name}/artifacts`"
                  class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-foreground hover:bg-surface-hover transition-colors"
                >
                  <ArrowUpTrayIcon class="w-4 h-4 text-muted" aria-hidden="true" />
                  Upload
                </router-link>
                <div class="w-px h-6 bg-border" />
                <button
                  class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-foreground hover:bg-surface-hover transition-colors"
                  @click="copyUrl"
                >
                  <ClipboardIcon class="w-4 h-4 text-muted" aria-hidden="true" />
                  Copy URL
                </button>
                <div class="w-px h-6 bg-border" />
                <router-link
                  :to="`/repos/${repo.name}/edit`"
                  class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-foreground hover:bg-surface-hover transition-colors"
                >
                  <Cog6ToothIcon class="w-4 h-4 text-muted" aria-hidden="true" />
                  Settings
                </router-link>
              </div>

              <button
                v-if="canDelete"
                @click="deleteRepo"
                :disabled="deleting"
                class="px-3 py-2 text-sm font-medium rounded-lg text-red-600 hover:bg-red-50 dark:hover:bg-red-950 transition-colors disabled:opacity-50 border border-transparent hover:border-red-200 dark:hover:border-red-900"
              >
                {{ deleting ? 'Deleting...' : 'Delete repository' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Tabs -->
        <div class="flex items-center gap-1 px-3 overflow-x-auto">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            class="relative h-11 px-3 text-sm font-medium transition-colors cursor-pointer shrink-0"
            :class="activeTab === tab.id ? 'text-foreground' : 'text-muted hover:text-foreground'"
            @click="activeTab = tab.id"
          >
            <span class="flex items-center gap-1.5">
              {{ tab.label }}
              <span
                v-if="tab.count !== undefined"
                class="text-[10px] font-mono px-1.5 py-0.5 rounded"
                :class="activeTab === tab.id
                  ? 'bg-primary-soft text-primary'
                  : 'bg-surface-hover text-muted'
                "
              >
                {{ tab.count }}
              </span>
            </span>
            <span
              v-if="activeTab === tab.id"
              class="absolute left-2 right-2 -bottom-px h-[2px] bg-primary rounded-full"
            />
          </button>
        </div>
      </div>

      <!-- Tab Content: Packages -->
      <div v-if="activeTab === 'cleanup'" class="mt-6">
        <RepoCleanupTab :repo-name="repoName" />
      </div>
      <div v-else-if="activeTab === 'packages'" class="mt-6">
        <div v-if="packages.length === 0" class="py-12">
          <EmptyState title="No packages" description="This repository has no packages yet." />
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b border-border">
                <th class="w-7 px-4 py-3 text-sm font-medium text-muted">
                  <input
                    type="checkbox"
                    :checked="allPkgsSelected"
                    :indeterminate="somePkgsSelected && !allPkgsSelected"
                    class="w-4 h-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
                    aria-label="Select all packages"
                    @change="toggleAllPkgs"
                  />
                </th>
                <th class="text-left px-4 py-3 text-sm font-medium text-muted">Name</th>
                <th class="text-left px-4 py-3 text-sm font-medium text-muted">Version</th>
                <th class="text-left px-4 py-3 text-sm font-medium text-muted">Format</th>
                <th class="text-left px-4 py-3 text-sm font-medium text-muted">Size</th>
                <th class="text-left px-4 py-3 text-sm font-medium text-muted">Date</th>
                <th class="text-right px-4 py-3 text-sm font-medium text-muted"></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="pkg in packages"
                :key="`${pkg.name}-${pkg.version}`"
                class="border-t border-border hover:bg-surface-hover transition-colors cursor-pointer group"
                :class="{ 'bg-primary-soft/40': selectedIds.has(itemKey(pkg)) }"
              >
                <td class="w-7 px-4 py-3" @click.stop>
                  <input
                    type="checkbox"
                    :checked="selectedIds.has(itemKey(pkg))"
                    class="w-4 h-4 rounded border-border text-primary focus:ring-primary cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity"
                    :class="{ 'opacity-100': selectedIds.has(itemKey(pkg)) }"
                    :aria-label="`Select ${pkg.name}`"
                    @change="togglePkg(pkg)"
                  />
                </td>
                <td class="px-4 py-3 text-sm font-medium">
                  <router-link
                    :to="`/repos/${repo!.name}/packages/${encodeURIComponent(pkg.format)}/${encodeURIComponent(pkg.name)}`"
                    class="text-foreground hover:text-primary transition-colors"
                    @click.stop
                  >{{ pkg.name }}</router-link>
                </td>
                <td class="px-4 py-3 text-xs font-mono text-muted">{{ pkg.version }}</td>
                <td class="px-4 py-3 text-sm">
                  <FormatBadge :format="pkg.format" />
                </td>
                <td class="px-4 py-3 text-sm text-muted">{{ formatSize(pkg.size) }}</td>
                <td class="px-4 py-3 text-sm text-muted">{{ formatDate(pkg.created_at) }}</td>
                <td class="px-4 py-3 text-right" @click.stop>
                  <div class="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <a
                      :href="`/api/v1/repositories/${encodeURIComponent(repo!.name)}/packages/${encodeURIComponent(pkg.format)}/${encodeURIComponent(pkg.name)}/download`"
                      class="w-7 h-7 flex items-center justify-center rounded border border-border text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                      download
                      :title="`Download ${pkg.name}`"
                    >
                      <ArrowDownTrayIcon class="w-3.5 h-3.5" aria-hidden="true" />
                    </a>
                    <button
                      class="w-7 h-7 flex items-center justify-center rounded border border-border text-muted hover:text-foreground hover:bg-surface-hover transition-colors text-danger"
                      :title="`Delete ${pkg.name}`"
                      @click="deletePkg(pkg)"
                    >
                      <TrashIcon class="w-3.5 h-3.5" aria-hidden="true" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Bulk actions bar -->
        <BulkActionsBar
          :selected-items="selectedItemKeys"
          :repo-name="repoName"
          @clear="clearSelection"
          @deleted="onDeleted"
          @error="msg => toasts.error(msg)"
        />
      </div>

      <!-- Tab Content: other tabs -->
      <div v-else class="mt-6">
        <div class="rounded-xl border border-border bg-surface p-8 text-center">
          <p class="text-sm text-muted">Configure {{ tabLabel(activeTab) }} for this repository.</p>
        </div>
      </div>

    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getRepository, listPackages, deleteRepository, deletePackage } from '../api/repositories'
import type { Repository, PackageItem } from '../api/repositories'
import FormatBadge from '../components/FormatBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import BulkActionsBar from '../components/BulkActionsBar.vue'
import RepoCleanupTab from './RepoCleanupTab.vue'
import { useToastStore } from '../stores/toast'
import { formatColor } from '../lib/formats'
import {
  ArrowUpTrayIcon,
  ClipboardIcon,
  Cog6ToothIcon,
  ArrowDownTrayIcon,
  TrashIcon,
} from '@heroicons/vue/24/outline'

const route = useRoute()
const router = useRouter()
const toasts = useToastStore()
const repo = ref<Repository | null>(null)
const packages = ref<PackageItem[]>([])
const totalPackages = ref(0)
const loading = ref(true)
const deleting = ref(false)
const activeTab = ref('packages')

const repoName = computed(() => route.params.name as string)
const repoFormatColor = computed(() => formatColor(repo.value?.format || ''))
const canDelete = computed(() => true)

// ── Selection state ────────────────────────────────────────────────────────
const selectedIds = ref<Set<string>>(new Set())

function itemKey(pkg: PackageItem): string {
  return `${pkg.format}/${pkg.name}`
}

const selectedItemKeys = computed(() => [...selectedIds.value])

const allPkgsSelected = computed(() =>
  packages.value.length > 0 &&
  packages.value.every((p) => selectedIds.value.has(itemKey(p))),
)

const somePkgsSelected = computed(() =>
  packages.value.some((p) => selectedIds.value.has(itemKey(p))),
)

function togglePkg(pkg: PackageItem): void {
  const key = itemKey(pkg)
  const next = new Set(selectedIds.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  selectedIds.value = next
}

function toggleAllPkgs(): void {
  if (allPkgsSelected.value) {
    const next = new Set(selectedIds.value)
    packages.value.forEach((p) => next.delete(itemKey(p)))
    selectedIds.value = next
  } else {
    const next = new Set(selectedIds.value)
    packages.value.forEach((p) => next.add(itemKey(p)))
    selectedIds.value = next
  }
}

function clearSelection(): void {
  selectedIds.value = new Set()
}

function onDeleted(ids: string[]): void {
  const deleted = new Set(ids)
  packages.value = packages.value.filter((p) => !deleted.has(itemKey(p)))
  totalPackages.value = Math.max(0, totalPackages.value - ids.length)
  clearSelection()
}

const tabs = computed(() => [
  { id: 'packages', label: 'Packages', count: totalPackages.value },
  { id: 'configuration', label: 'Configuration' },
  { id: 'permissions', label: 'Permissions' },
  { id: 'webhooks', label: 'Webhooks' },
  { id: 'cleanup', label: 'Cleanup policies' },
  { id: 'audit', label: 'Audit' },
])

function tabLabel(id: string): string {
  return tabs.value.find((t) => t.id === id)?.label || id
}

function formatSize(bytes?: number): string {
  if (!bytes) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }
  return `${size.toFixed(2)} ${units[unitIndex]}`
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    return date.toLocaleDateString()
  } catch {
    return dateStr
  }
}

function copyUrl(): void {
  if (!repo.value) return
  const url = `${window.location.origin}/${repo.value.name}/`
  navigator.clipboard.writeText(url).then(() => {
    toasts.success('URL copied to clipboard')
  }).catch(() => {
    toasts.error('Failed to copy URL')
  })
}

async function deleteRepo() {
  if (!repo.value) return
  if (!confirm(`Are you sure you want to delete "${repo.value.name}"?`)) return

  deleting.value = true
  try {
    await deleteRepository(repo.value.name)
    router.push('/repos')
  } catch {
    toasts.error('Failed to delete repository')
  } finally {
    deleting.value = false
  }
}

async function deletePkg(pkg: PackageItem): Promise<void> {
  if (!repo.value) return
  if (!confirm(`Delete "${pkg.name}"?`)) return
  try {
    await deletePackage(repo.value.name, pkg.format, pkg.name)
    packages.value = packages.value.filter(p => !(p.name === pkg.name && p.format === pkg.format))
    totalPackages.value = Math.max(0, totalPackages.value - 1)
    toasts.success(`Deleted ${pkg.name}`)
  } catch {
    toasts.error('Failed to delete package')
  }
}

watch(activeTab, () => nextTick())

onMounted(async () => {
  try {
    const [repoRes, pkgRes] = await Promise.all([
      getRepository(repoName.value),
      listPackages(repoName.value, { limit: 20 }),
    ])
    repo.value = repoRes
    packages.value = pkgRes.packages || []
    totalPackages.value = pkgRes.total || 0
  } finally {
    loading.value = false
    nextTick()
  }
})

// ── HeroStat inline component ──────────────────────────────────────────────
const HeroStat = {
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    mono: { type: Boolean, default: false },
  },
  setup(props: { label: string; value: string; mono: boolean }) {
    return () =>
      h('div', [
        h('div', { class: 'text-[10px] uppercase tracking-[0.14em] text-muted font-semibold' }, props.label),
        h(
          'div',
          { class: `mt-0.5 text-sm font-medium tabular-nums ${props.mono ? 'font-mono' : ''}` },
          props.value,
        ),
      ])
  },
}
</script>
