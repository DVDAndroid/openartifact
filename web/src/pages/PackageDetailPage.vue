<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div>
    <!-- Back Button -->
    <button
      @click="router.back()"
      class="flex items-center gap-1 text-sm text-muted hover:text-foreground transition-colors mb-4 cursor-pointer"
    >
      <ChevronLeftIcon class="w-4 h-4" aria-hidden="true" />
      Back to repository
    </button>

    <div v-if="loading" class="text-muted text-sm">Loading...</div>
    <EmptyState
      v-else-if="!currentPkg"
      title="Package not found"
      description="This package could not be found in the repository."
    />
    <div v-else>
      <!-- Hero Card -->
      <div class="bg-surface rounded-xl border border-border overflow-hidden">
        <!-- Top section -->
        <div class="relative px-6 py-5 border-b border-border">
          <!-- Radial gradient overlay -->
          <div
            class="absolute inset-0 opacity-[0.08] pointer-events-none"
            style="background: radial-gradient(60% 40% at 0% 0%, rgb(var(--color-primary) / 0.08), transparent 70%)"
          />

          <div class="relative">
            <!-- Title row + actions -->
            <div class="flex items-start justify-between gap-4 flex-wrap">
              <div class="flex items-center gap-2.5 flex-wrap">
                <h1 class="text-2xl font-bold tracking-tight text-foreground">{{ pkgName }}</h1>
                <FormatBadge :format="format" />
                <span class="font-mono text-xs bg-surface-hover rounded-md px-2 py-0.5 border border-border text-muted">
                  {{ currentPkg.version }}
                </span>
                <span
                  v-if="packages.length > 0 && currentPkg?.version === packages[0]?.version"
                  class="text-[10px] font-medium px-2 py-0.5 rounded-full border border-green-500/30 text-green-600 dark:text-green-400 bg-green-500/10"
                >
                  latest
                </span>
              </div>

              <!-- Action group -->
              <div class="inline-flex items-center rounded-lg border border-border bg-surface overflow-hidden shrink-0 shadow-sm">
                <button
                  class="inline-flex items-center gap-1.5 px-3 h-9 text-sm text-foreground/80 hover:bg-surface-hover transition-colors"
                  @click="router.push(`/repos/${repoName}/artifacts`)"
                >
                  <ArrowUpTrayIcon class="w-4 h-4 text-muted" aria-hidden="true" />
                  Upload
                </button>
                <div class="w-px h-6 bg-border" />
                <button
                  class="inline-flex items-center gap-1.5 px-3 h-9 text-sm text-foreground/80 hover:bg-surface-hover transition-colors"
                  @click="copyUrl"
                >
                  <ClipboardDocumentIcon class="w-4 h-4 text-muted" aria-hidden="true" />
                  Copy URL
                </button>
                <div class="w-px h-6 bg-border" />
                <button
                  class="inline-flex items-center gap-1.5 px-3 h-9 text-sm text-foreground/80 hover:bg-surface-hover transition-colors"
                  @click="router.push(`/repos/${repoName}/edit`)"
                >
                  <Cog6ToothIcon class="w-4 h-4 text-muted" aria-hidden="true" />
                  Settings
                </button>
              </div>
            </div>

            <!-- Description -->
            <p class="text-sm text-muted max-w-2xl leading-relaxed mt-2">
              {{ repo?.description || `Repository: ${repoName}` }}
            </p>

            <!-- Stats row -->
            <div class="flex items-center gap-5 flex-wrap mt-3.5">
              <div class="leading-tight">
                <div class="text-[10px] uppercase tracking-[0.14em] text-muted font-semibold">Format</div>
                <div class="mt-0.5 text-sm font-medium tabular-nums">{{ format }}</div>
              </div>
              <div v-if="currentPkg.size" class="leading-tight">
                <div class="text-[10px] uppercase tracking-[0.14em] text-muted font-semibold">Size</div>
                <div class="mt-0.5 text-sm font-medium tabular-nums font-mono">{{ formatSize(currentPkg.size) }}</div>
              </div>
              <div v-if="currentPkg.created_at" class="leading-tight">
                <div class="text-[10px] uppercase tracking-[0.14em] text-muted font-semibold">Published</div>
                <div class="mt-0.5 text-sm font-medium tabular-nums">{{ new Date(currentPkg.created_at).toLocaleDateString() }}</div>
              </div>
              <div v-if="currentPkg.sha256" class="leading-tight">
                <div class="text-[10px] uppercase tracking-[0.14em] text-muted font-semibold">SHA256</div>
                <div class="mt-0.5 text-sm font-medium tabular-nums font-mono">{{ currentPkg.sha256.slice(0, 12) }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- Tabs (inside the card) -->
        <div class="flex items-center gap-1 px-3 overflow-x-auto">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="relative h-11 px-3 text-sm font-medium transition-colors cursor-pointer shrink-0"
            :class="activeTab === tab.key ? 'text-foreground' : 'text-muted hover:text-foreground'"
            @click="activeTab = tab.key"
          >
            <span class="inline-flex items-center gap-1.5">
              {{ tab.label }}
              <span
                v-if="tab.key === 'versions' && packages.length > 0"
                class="text-[10px] font-mono px-1.5 py-0.5 rounded"
                :class="activeTab === 'versions' ? 'bg-primary-soft text-primary' : 'bg-surface-hover text-muted'"
              >
                {{ packages.length }}
              </span>
              <span
                v-if="tab.key === 'files' && components.length > 0"
                class="text-[10px] font-mono px-1.5 py-0.5 rounded"
                :class="activeTab === 'files' ? 'bg-primary-soft text-primary' : 'bg-surface-hover text-muted'"
              >
                {{ components.length }}
              </span>
            </span>
            <span
              v-if="activeTab === tab.key"
              class="absolute left-2 right-2 -bottom-px h-[2px] bg-primary rounded-full"
            />
          </button>
        </div>
      </div>

      <!-- Pull Command -->
      <div class="bg-[#0d1117] rounded-lg border border-border/50 p-3 mt-4 flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" />
        <code class="font-mono text-xs text-foreground/90 flex-1 truncate">{{ pullCommand }}</code>
        <button
          @click="copyCommand"
          class="flex-shrink-0 text-muted hover:text-foreground transition-colors"
          :title="copied ? 'Copied!' : 'Copy to clipboard'"
        >
          <ClipboardDocumentIcon class="w-4 h-4" aria-hidden="true" />
        </button>
      </div>

      <!-- Tab Content: Overview -->
      <div v-if="activeTab === 'overview'" class="mt-6">
        <div class="bg-surface rounded-lg border border-border p-6">
          <div class="space-y-0">
            <div class="flex items-center justify-between py-2.5 border-b border-border">
              <span class="text-sm text-muted">Name</span>
              <span class="text-sm text-foreground font-mono">{{ currentPkg.name }}</span>
            </div>
            <div class="flex items-center justify-between py-2.5 border-b border-border">
              <span class="text-sm text-muted">Version</span>
              <span class="text-sm text-foreground font-mono">{{ currentPkg.version }}</span>
            </div>
            <div class="flex items-center justify-between py-2.5 border-b border-border">
              <span class="text-sm text-muted">Format</span>
              <span class="text-sm text-foreground font-mono">{{ format }}</span>
            </div>
            <div v-if="currentPkg.size" class="flex items-center justify-between py-2.5 border-b border-border">
              <span class="text-sm text-muted">Size</span>
              <span class="text-sm text-foreground font-mono">{{ formatSize(currentPkg.size) }}</span>
            </div>
            <div v-if="currentPkg.sha256" class="flex items-center justify-between py-2.5 border-b border-border">
              <span class="text-sm text-muted">SHA256</span>
              <span class="text-sm text-foreground font-mono">{{ currentPkg.sha256 }}</span>
            </div>
            <div v-if="currentPkg.created_at" class="flex items-center justify-between py-2.5">
              <span class="text-sm text-muted">Created</span>
              <span class="text-sm text-foreground font-mono">{{ new Date(currentPkg.created_at).toLocaleDateString() }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Tab Content: Versions -->
      <div v-if="activeTab === 'versions'" class="mt-6">
        <div class="bg-surface rounded-lg border border-border overflow-hidden">
          <table class="w-full text-sm">
            <thead class="border-b border-border bg-surface-hover/40">
              <tr>
                <th class="px-4 py-3 text-left text-muted font-medium">Version</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Size</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Published</th>
                <th class="px-4 py-3 text-left text-muted font-medium">SHA256</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="(pkg, idx) in packages"
                :key="pkg.version"
                class="hover:bg-surface-hover/40 transition-colors"
              >
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <span class="text-foreground font-mono text-sm">{{ pkg.version }}</span>
                    <span
                      v-if="idx === 0"
                      class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-primary-soft text-primary"
                    >latest</span>
                  </div>
                </td>
                <td class="px-4 py-3 text-foreground font-mono text-xs tabular-nums">{{ pkg.size ? formatSize(pkg.size) : '—' }}</td>
                <td class="px-4 py-3 text-foreground text-xs">{{ pkg.created_at ? new Date(pkg.created_at).toLocaleDateString() : '—' }}</td>
                <td class="px-4 py-3 text-foreground font-mono text-xs">{{ pkg.sha256 ? pkg.sha256.slice(0, 12) : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Tab Content: Files -->
      <div v-if="activeTab === 'files'" class="mt-6">
        <div v-if="components.length > 0" class="bg-surface rounded-lg border border-border overflow-hidden">
          <table class="w-full text-sm">
            <thead class="border-b border-border bg-surface-hover/40">
              <tr>
                <th class="px-4 py-3 text-left text-muted font-medium">Path/Name</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Version</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Size</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="comp in components"
                :key="comp.path || comp.name"
                class="hover:bg-surface-hover/40 transition-colors"
              >
                <td class="px-4 py-3 text-foreground font-mono text-xs">{{ comp.path || comp.name }}</td>
                <td class="px-4 py-3 text-foreground font-mono text-xs">{{ comp.version || '—' }}</td>
                <td class="px-4 py-3 text-foreground font-mono text-xs tabular-nums">{{ comp.size ? formatSize(comp.size) : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <EmptyState v-else title="No files" description="This package has no components or files." />
      </div>

      <!-- Tab Content: Properties -->
      <div v-if="activeTab === 'properties'" class="mt-6">
        <div class="bg-surface rounded-lg border border-border overflow-hidden">
          <!-- Header -->
          <div class="px-5 py-4 flex items-center justify-between border-b border-border">
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-semibold text-foreground">Custom Properties</h3>
              <span
                v-if="Object.keys(properties).length > 0"
                class="px-1.5 py-0.5 rounded-md bg-surface-hover text-[11px] font-mono font-medium text-muted"
              >
                {{ Object.keys(properties).length }}
              </span>
            </div>
            <button
              type="button"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
              @click="addingProp = true"
            >
              <PlusIcon class="w-4 h-4" />
              Add property
            </button>
          </div>

          <!-- Add property inline form -->
          <div v-if="addingProp" class="px-5 py-4 border-b border-border bg-surface-hover/20">
            <div class="flex items-center gap-2">
              <input
                v-model="newPropKey"
                type="text"
                placeholder="Key"
                class="flex-1 px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                @keydown.enter="saveNewProp"
                @keydown.escape="cancelAddProp"
              />
              <input
                v-model="newPropValue"
                type="text"
                placeholder="Value"
                class="flex-1 px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                @keydown.enter="saveNewProp"
                @keydown.escape="cancelAddProp"
              />
              <button
                type="button"
                :disabled="!newPropKey.trim() || propsSaving"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed shrink-0"
                @click="saveNewProp"
              >
                <ArrowPathIcon v-if="propsSaving" class="w-4 h-4 animate-spin" />
                <CheckIcon v-else class="w-4 h-4" />
                Add
              </button>
              <button
                type="button"
                class="p-2 rounded-lg text-muted hover:text-foreground hover:bg-surface-hover transition-colors shrink-0"
                @click="cancelAddProp"
              >
                <XMarkIcon class="w-4 h-4" />
              </button>
            </div>
          </div>

          <!-- Properties loading -->
          <div v-if="propsLoading" class="px-5 py-6 text-center text-sm text-muted">
            Loading properties…
          </div>

          <!-- Empty state -->
          <div
            v-else-if="Object.keys(properties).length === 0 && !addingProp"
            class="py-12 text-center"
          >
            <div class="w-10 h-10 rounded-full bg-surface-hover mx-auto flex items-center justify-center mb-3">
              <TagIcon class="w-5 h-5 text-muted" />
            </div>
            <p class="text-sm font-medium text-foreground">No custom properties</p>
            <p class="text-xs text-muted mt-1 mb-4">Attach key-value metadata to this artifact.</p>
            <button
              type="button"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
              @click="addingProp = true"
            >
              <PlusIcon class="w-4 h-4" />
              Add property
            </button>
          </div>

          <!-- Properties table -->
          <table v-else-if="Object.keys(properties).length > 0" class="w-full">
            <thead>
              <tr class="bg-surface-hover/30">
                <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5 w-1/3">
                  Key
                </th>
                <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
                  Value
                </th>
                <th class="w-10" />
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="(value, key) in properties"
                :key="key"
                class="hover:bg-surface-hover/20 transition-colors group"
              >
                <td class="px-5 py-3">
                  <span class="font-mono text-[12px] font-medium text-foreground">{{ key }}</span>
                </td>
                <td class="px-3 py-3">
                  <!-- Inline edit mode -->
                  <template v-if="editingKey === key">
                    <div class="flex items-center gap-2">
                      <input
                        v-model="editingValue"
                        type="text"
                        class="flex-1 px-2 py-1 rounded border border-primary bg-bg text-foreground text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                        @keydown.enter="saveEditProp(String(key))"
                        @keydown.escape="cancelEditProp"
                        autofocus
                      />
                      <button
                        type="button"
                        class="p-1 rounded text-success hover:bg-success/10 transition-colors shrink-0"
                        @click="saveEditProp(String(key))"
                      >
                        <CheckIcon class="w-4 h-4" />
                      </button>
                      <button
                        type="button"
                        class="p-1 rounded text-muted hover:text-foreground hover:bg-surface-hover transition-colors shrink-0"
                        @click="cancelEditProp"
                      >
                        <XMarkIcon class="w-4 h-4" />
                      </button>
                    </div>
                  </template>
                  <!-- Display mode -->
                  <button
                    v-else
                    type="button"
                    class="font-mono text-[12px] text-foreground/80 hover:text-primary transition-colors text-left w-full truncate"
                    title="Click to edit"
                    @click="startEditProp(String(key), String(value))"
                  >
                    {{ value }}
                  </button>
                </td>
                <td class="px-3 py-3">
                  <button
                    type="button"
                    title="Delete property"
                    class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger/10 transition-colors opacity-0 group-hover:opacity-100"
                    @click="deleteProp(String(key))"
                  >
                    <TrashIcon class="w-4 h-4" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Tab Content: Security -->
      <div v-if="activeTab === 'security'" class="mt-6">
        <div class="flex flex-col items-center justify-center py-12 text-center">
          <ShieldCheckIcon class="w-12 h-12 text-success mb-4" />
          <h3 class="text-lg font-semibold text-foreground">No vulnerabilities found</h3>
          <p class="text-sm text-muted mt-1 max-w-md">
            This package version has been scanned and no known CVEs were detected.
          </p>
        </div>

        <div class="bg-surface rounded-lg border border-border overflow-hidden mt-6">
          <table class="w-full text-sm">
            <thead class="border-b border-border bg-surface-hover/40">
              <tr>
                <th class="px-4 py-3 text-left text-muted font-medium">CVE ID</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Severity</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Description</th>
                <th class="px-4 py-3 text-left text-muted font-medium">Fixed In</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td colspan="4" class="px-4 py-8">
                  <EmptyState title="No vulnerabilities" description="No known CVEs have been reported for this package version." />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Tab Content: Dependencies -->
      <div v-if="activeTab === 'dependencies'" class="mt-6">
        <div class="bg-surface rounded-lg border border-border p-6">
          <h3 class="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
            <PuzzlePieceIcon class="w-4 h-4" />
            Dependencies
          </h3>
          <EmptyState title="No dependencies found" description="Dependency information is not available for this package format." />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ChevronLeftIcon,
  ClipboardDocumentIcon,
  ShieldCheckIcon,
  PuzzlePieceIcon,
  ArrowUpTrayIcon,
  Cog6ToothIcon,
  PlusIcon,
  CheckIcon,
  XMarkIcon,
  TrashIcon,
  TagIcon,
  ArrowPathIcon,
} from '@heroicons/vue/24/outline'
import { getRepository, listPackages, listComponents } from '../api/repositories'
import type { Repository, PackageItem, ComponentItem } from '../api/repositories'
import { request, ApiError } from '../api/client'
import { useToastStore } from '../stores/toast'
import FormatBadge from '../components/FormatBadge.vue'
import EmptyState from '../components/EmptyState.vue'

const tabs = [
  { key: 'overview', label: 'Overview' },
  { key: 'versions', label: 'Versions' },
  { key: 'files', label: 'Files' },
  { key: 'properties', label: 'Properties' },
  { key: 'security', label: 'Security' },
  { key: 'dependencies', label: 'Dependencies' },
] as const

const route = useRoute()
const router = useRouter()

const repoName = computed(() => route.params.name as string)
const format = computed(() => route.params.format as string)
const pkgName = computed(() => route.params.pkg as string)

const toasts = useToastStore()
const repo = ref<Repository | null>(null)
const packages = ref<PackageItem[]>([])
const components = ref<ComponentItem[]>([])
const loading = ref(true)
const activeTab = ref('overview')
const copied = ref(false)

// ─── Properties state ─────────────────────────────────────────────────────────
const properties = ref<Record<string, string>>({})
const propsLoading = ref(false)
const propsSaving = ref(false)
const addingProp = ref(false)
const newPropKey = ref('')
const newPropValue = ref('')
const editingKey = ref<string | null>(null)
const editingValue = ref('')

const artifactId = computed(() => currentPkg.value?.sha256 || currentPkg.value?.name || '')

async function loadProperties(): Promise<void> {
  if (!artifactId.value) return
  propsLoading.value = true
  try {
    const res = await request<{ properties: Record<string, string> }>(
      `/api/v1/repositories/${encodeURIComponent(repoName.value)}/artifacts/${encodeURIComponent(artifactId.value)}/properties`,
    )
    properties.value = res?.properties ?? {}
  } catch (err) {
    if (!(err instanceof ApiError && (err.status === 404 || err.status === 501))) {
      toasts.error('Failed to load properties')
    }
    properties.value = {}
  } finally {
    propsLoading.value = false
  }
}

function cancelAddProp(): void {
  addingProp.value = false
  newPropKey.value = ''
  newPropValue.value = ''
}

async function saveNewProp(): Promise<void> {
  const key = newPropKey.value.trim()
  if (!key || !artifactId.value) return
  propsSaving.value = true
  try {
    const updated = { ...properties.value, [key]: newPropValue.value }
    await request<void>(
      `/api/v1/repositories/${encodeURIComponent(repoName.value)}/artifacts/${encodeURIComponent(artifactId.value)}/properties`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ properties: { [key]: newPropValue.value } }),
      },
    )
    properties.value = updated
    cancelAddProp()
    toasts.success('Property added')
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to add property')
  } finally {
    propsSaving.value = false
  }
}

function startEditProp(key: string, value: string): void {
  editingKey.value = key
  editingValue.value = value
}

function cancelEditProp(): void {
  editingKey.value = null
  editingValue.value = ''
}

async function saveEditProp(key: string): Promise<void> {
  if (!artifactId.value) return
  try {
    await request<void>(
      `/api/v1/repositories/${encodeURIComponent(repoName.value)}/artifacts/${encodeURIComponent(artifactId.value)}/properties`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ properties: { [key]: editingValue.value } }),
      },
    )
    properties.value = { ...properties.value, [key]: editingValue.value }
    cancelEditProp()
    toasts.success('Property updated')
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to update property')
  }
}

async function deleteProp(key: string): Promise<void> {
  if (!artifactId.value) return
  if (!window.confirm(`Delete property "${key}"?`)) return
  try {
    await request<void>(
      `/api/v1/repositories/${encodeURIComponent(repoName.value)}/artifacts/${encodeURIComponent(artifactId.value)}/properties/${encodeURIComponent(key)}`,
      { method: 'DELETE' },
    )
    const next = { ...properties.value }
    delete next[key]
    properties.value = next
    toasts.success('Property deleted')
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to delete property')
  }
}

watch(activeTab, (tab) => {
  if (tab === 'properties' && !propsLoading.value && artifactId.value) {
    void loadProperties()
  }
})

const currentPkg = computed(() => packages.value[0] || null)

const pullCommand = computed(() => {
  if (!currentPkg.value) return ''
  const fmt = format.value
  const repo = repoName.value
  const pkg = pkgName.value
  const ver = currentPkg.value.version

  switch (fmt) {
    case 'docker':
      return `docker pull ${repo}/${pkg}:${ver}`
    case 'npm':
      return `npm install ${pkg}@${ver} --registry=...`
    case 'pypi':
      return `pip install ${pkg}==${ver} --index-url=...`
    case 'maven':
      return `mvn dependency:get -Dartifact=${pkg}:${ver}`
    case 'helm':
      return `helm pull ${repo}/${pkg} --version ${ver}`
    case 'cargo':
      return `cargo install ${pkg}@${ver}`
    default:
      return `# Pull ${pkg}@${ver} from ${repo}`
  }
})

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

async function copyUrl(): Promise<void> {
  try {
    await navigator.clipboard.writeText(`/${repoName.value}/${pkgName.value}/`)
  } catch {
    // best-effort
  }
}

async function copyCommand(): Promise<void> {
  try {
    await navigator.clipboard.writeText(pullCommand.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    // best-effort
  }
}

onMounted(async () => {
  try {
    const [repoRes, pkgRes, compRes] = await Promise.all([
      getRepository(repoName.value),
      listPackages(repoName.value, { limit: 100 }),
      listComponents(repoName.value, { prefix: pkgName.value }).catch(() => ({ total: 0, items: [] })),
    ])
    repo.value = repoRes
    packages.value = (pkgRes.packages || []).filter(p => p.name === pkgName.value)
    components.value = compRes.items || []
  } finally {
    loading.value = false
  }
})
</script>
