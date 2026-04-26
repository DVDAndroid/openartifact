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
          <span class="text-foreground/70">AI Analysis</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">AI Analysis</h1>
        <p class="mt-1 text-sm text-muted">AI-powered artifact analysis for license compliance, security notes, and quarantine recommendations.</p>
      </div>
      <button
        class="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50"
        :disabled="!selectedRepo || triggering"
        @click="openTriggerDrawer"
      >
        <SparklesIcon class="w-4 h-4" />
        Trigger Analysis
      </button>
    </div>

    <!-- Repo selector + refresh -->
    <div class="flex items-center gap-3">
      <select
        v-model="selectedRepo"
        class="h-9 px-3 rounded-lg bg-surface border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
        @change="loadAnalyses"
      >
        <option value="">Select a repository…</option>
        <option v-for="r in repos" :key="r.name" :value="r.name">{{ r.name }}</option>
      </select>
      <button
        class="flex items-center justify-center w-9 h-9 rounded-lg border border-border text-muted hover:text-foreground hover:bg-surface-hover transition-colors disabled:opacity-50"
        :disabled="loadingAnalyses || !selectedRepo"
        @click="loadAnalyses"
      >
        <ArrowPathIcon class="w-4 h-4" :class="{ 'animate-spin': loadingAnalyses }" />
      </button>
    </div>

    <!-- Stats tiles -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Total Analyses</div>
        <div class="text-2xl font-bold mt-1 tabular-nums">{{ analyses.length }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Safe</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-success">{{ countSafe }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Review Required</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-warning">{{ countReview }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Quarantine Recommended</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-danger">{{ countQuarantine }}</div>
      </div>
    </div>

    <!-- Filter toolbar -->
    <div class="flex items-center gap-3 flex-wrap">
      <div class="relative flex-1 max-w-sm">
        <MagnifyingGlassIcon class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
        <input
          v-model="searchQuery"
          type="search"
          placeholder="Search artifacts…"
          class="w-full pl-9 pr-3 h-9 text-sm rounded-lg bg-surface-hover/40 border border-border text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
        />
      </div>
      <!-- Status filter -->
      <div class="flex items-center gap-1 bg-surface-hover/60 rounded-lg p-0.5">
        <button
          v-for="s in statusOptions"
          :key="s.value"
          class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
          :class="statusFilter === s.value ? 'bg-surface text-foreground shadow-sm' : 'text-muted hover:text-foreground'"
          @click="statusFilter = s.value"
        >{{ s.label }}</button>
      </div>
      <!-- License filter -->
      <div class="flex items-center gap-1 bg-surface-hover/60 rounded-lg p-0.5">
        <button
          v-for="l in licenseOptions"
          :key="l.value"
          class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
          :class="licenseFilter === l.value ? 'bg-surface text-foreground shadow-sm' : 'text-muted hover:text-foreground'"
          @click="licenseFilter = l.value"
        >{{ l.label }}</button>
      </div>
    </div>

    <!-- Results table -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Analysis Results</h3>
        <span class="text-[11px] font-mono text-muted">{{ filteredAnalyses.length }} analyses</span>
      </div>

      <div v-if="loadingAnalyses" class="py-12 flex flex-col items-center gap-3 text-muted">
        <ArrowPathIcon class="w-6 h-6 animate-spin" />
        <span class="text-sm">Loading analyses…</span>
      </div>

      <EmptyState
        v-else-if="!selectedRepo"
        title="Select a repository"
        description="Choose a repository above to view its AI analyses."
      />

      <EmptyState
        v-else-if="filteredAnalyses.length === 0"
        title="No AI analyses found"
        description="Trigger an analysis using the button above to get AI-powered insights."
      />

      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="border-b border-border bg-surface-hover/40">
            <tr>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Artifact</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Format</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">License</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Status</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Quarantine</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Summary</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="analysis in filteredAnalyses"
              :key="analysis.id"
              class="hover:bg-surface-hover/40 transition-colors border-b border-border last:border-0 cursor-pointer"
              @click="openDetail(analysis)"
            >
              <td class="px-4 py-3">
                <div class="font-mono text-[13px] font-medium text-foreground">{{ analysis.artifact_name }}</div>
                <div class="font-mono text-[11px] text-muted mt-0.5">{{ analysis.artifact_version }}</div>
              </td>
              <td class="px-4 py-3">
                <FormatBadge :format="analysis.artifact_format" />
              </td>
              <td class="px-4 py-3">
                <div class="flex flex-col gap-1">
                  <span :class="licenseStatusClass(analysis.license_status)">
                    {{ licenseStatusLabel(analysis.license_status) }}
                  </span>
                  <span v-if="analysis.license_name" class="text-[11px] text-muted font-mono truncate max-w-[140px]">{{ analysis.license_name }}</span>
                </div>
              </td>
              <td class="px-4 py-3">
                <span :class="analysisStatusClass(analysis.status)">
                  <ArrowPathIcon v-if="analysis.status === 'processing'" class="w-3 h-3 animate-spin" />
                  {{ analysis.status }}
                </span>
              </td>
              <td class="px-4 py-3">
                <span
                  v-if="analysis.quarantine_recommended"
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-danger bg-danger-soft"
                >
                  <ExclamationTriangleIcon class="w-3 h-3" />
                  Quarantine
                </span>
                <span v-else class="text-muted text-xs">—</span>
              </td>
              <td class="px-4 py-3 text-xs text-muted max-w-[200px]">
                <span class="line-clamp-2">{{ truncate(analysis.summary, 60) }}</span>
              </td>
              <td class="px-4 py-3" @click.stop>
                <button
                  class="px-2 py-1 rounded-md text-[11px] font-medium border border-border text-muted hover:bg-surface-hover hover:text-foreground transition-colors disabled:opacity-50"
                  :disabled="retriggering === analysis.id"
                  @click="retrigger(analysis)"
                >
                  <ArrowPathIcon v-if="retriggering === analysis.id" class="w-3 h-3 animate-spin inline mr-1" />
                  Re-trigger
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Detail drawer -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div v-if="selectedAnalysis" class="fixed inset-0 z-50 flex justify-end" @click.self="selectedAnalysis = null">
          <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="selectedAnalysis = null" />
          <Transition
            enter-active-class="transition-transform duration-300 ease-out"
            enter-from-class="translate-x-full"
            enter-to-class="translate-x-0"
            leave-active-class="transition-transform duration-200 ease-in"
            leave-from-class="translate-x-0"
            leave-to-class="translate-x-full"
          >
            <div v-if="selectedAnalysis" class="relative w-full max-w-lg h-full bg-surface border-l border-border shadow-2xl flex flex-col">
              <!-- Drawer header -->
              <div class="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
                <div class="min-w-0">
                  <h2 class="text-base font-semibold truncate">{{ selectedAnalysis.artifact_name }}</h2>
                  <div class="flex items-center gap-2 mt-0.5">
                    <span class="text-xs text-muted font-mono">{{ selectedAnalysis.artifact_version }}</span>
                    <FormatBadge :format="selectedAnalysis.artifact_format" />
                  </div>
                </div>
                <button
                  class="w-8 h-8 flex items-center justify-center rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors ml-3 shrink-0"
                  @click="selectedAnalysis = null"
                >
                  <XMarkIcon class="w-4 h-4" />
                </button>
              </div>

              <!-- Drawer body -->
              <div class="flex-1 overflow-y-auto px-6 py-5 space-y-5">
                <!-- Status + quarantine row -->
                <div class="flex items-center gap-3 flex-wrap">
                  <span :class="analysisStatusClass(selectedAnalysis.status)">
                    <ArrowPathIcon v-if="selectedAnalysis.status === 'processing'" class="w-3 h-3 animate-spin" />
                    {{ selectedAnalysis.status }}
                  </span>
                  <span
                    v-if="selectedAnalysis.quarantine_recommended"
                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-danger bg-danger-soft"
                  >
                    <ExclamationTriangleIcon class="w-3 h-3" />
                    Quarantine Recommended
                  </span>
                  <span
                    v-else
                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-success bg-success-soft/60"
                  >
                    No Quarantine
                  </span>
                </div>

                <!-- Error message -->
                <div v-if="selectedAnalysis.error_message" class="rounded-lg bg-danger-soft border border-danger/20 px-4 py-3">
                  <p class="text-xs font-semibold text-danger mb-1">Error</p>
                  <p class="text-xs text-danger/80 font-mono">{{ selectedAnalysis.error_message }}</p>
                </div>

                <!-- Summary -->
                <div v-if="selectedAnalysis.summary">
                  <h4 class="text-xs font-semibold text-muted uppercase tracking-[0.12em] mb-2">Summary</h4>
                  <p class="text-sm text-foreground leading-relaxed">{{ selectedAnalysis.summary }}</p>
                </div>

                <!-- Tags -->
                <div v-if="selectedAnalysis.tags && selectedAnalysis.tags.length > 0">
                  <h4 class="text-xs font-semibold text-muted uppercase tracking-[0.12em] mb-2">Tags</h4>
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="tag in selectedAnalysis.tags"
                      :key="tag"
                      class="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-medium bg-primary/10 text-primary border border-primary/20"
                    >
                      {{ tag }}
                    </span>
                  </div>
                </div>

                <!-- License -->
                <div>
                  <h4 class="text-xs font-semibold text-muted uppercase tracking-[0.12em] mb-2">License</h4>
                  <div class="flex items-center gap-2 flex-wrap">
                    <span :class="licenseStatusClass(selectedAnalysis.license_status)">
                      {{ licenseStatusLabel(selectedAnalysis.license_status) }}
                    </span>
                    <span v-if="selectedAnalysis.license_name" class="text-sm text-foreground font-mono">{{ selectedAnalysis.license_name }}</span>
                    <span v-else class="text-sm text-muted">Unknown license</span>
                  </div>
                </div>

                <!-- Security notes -->
                <div v-if="selectedAnalysis.security_notes">
                  <h4 class="text-xs font-semibold text-muted uppercase tracking-[0.12em] mb-2">Security Notes</h4>
                  <p class="text-sm text-foreground leading-relaxed whitespace-pre-wrap">{{ selectedAnalysis.security_notes }}</p>
                </div>

                <!-- Model info -->
                <div class="rounded-lg bg-surface-hover/60 border border-border px-4 py-3 space-y-2">
                  <h4 class="text-xs font-semibold text-muted uppercase tracking-[0.12em]">Model Information</h4>
                  <div class="grid grid-cols-2 gap-2 text-xs">
                    <div>
                      <span class="text-muted">Model</span>
                      <p class="font-mono text-foreground mt-0.5">{{ selectedAnalysis.model_used || '—' }}</p>
                    </div>
                    <div>
                      <span class="text-muted">Prompt tokens</span>
                      <p class="font-mono text-foreground mt-0.5 tabular-nums">{{ selectedAnalysis.prompt_tokens ?? '—' }}</p>
                    </div>
                    <div>
                      <span class="text-muted">Completion tokens</span>
                      <p class="font-mono text-foreground mt-0.5 tabular-nums">{{ selectedAnalysis.completion_tokens ?? '—' }}</p>
                    </div>
                    <div>
                      <span class="text-muted">Updated</span>
                      <p class="font-mono text-foreground mt-0.5">{{ formatDate(selectedAnalysis.updated_at) }}</p>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Drawer footer -->
              <div class="px-6 py-4 border-t border-border flex items-center justify-end gap-2 shrink-0">
                <button
                  class="px-4 py-2 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
                  @click="selectedAnalysis = null"
                >
                  Close
                </button>
                <button
                  class="flex items-center gap-2 px-4 py-2 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50"
                  :disabled="retriggering === selectedAnalysis.id"
                  @click="retrigger(selectedAnalysis)"
                >
                  <ArrowPathIcon v-if="retriggering === selectedAnalysis.id" class="w-3.5 h-3.5 animate-spin" />
                  <SparklesIcon v-else class="w-3.5 h-3.5" />
                  {{ retriggering === selectedAnalysis.id ? 'Triggering…' : 'Re-trigger Analysis' }}
                </button>
              </div>
            </div>
          </Transition>
        </div>
      </Transition>
    </Teleport>

    <!-- Trigger drawer (select artifact) -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div v-if="showTriggerDrawer" class="fixed inset-0 z-50 flex justify-end" @click.self="showTriggerDrawer = false">
          <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="showTriggerDrawer = false" />
          <Transition
            enter-active-class="transition-transform duration-300 ease-out"
            enter-from-class="translate-x-full"
            enter-to-class="translate-x-0"
            leave-active-class="transition-transform duration-200 ease-in"
            leave-from-class="translate-x-0"
            leave-to-class="translate-x-full"
          >
            <div v-if="showTriggerDrawer" class="relative w-full max-w-md h-full bg-surface border-l border-border shadow-2xl flex flex-col">
              <!-- Drawer header -->
              <div class="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
                <div>
                  <h2 class="text-base font-semibold">Trigger AI Analysis</h2>
                  <p class="text-xs text-muted mt-0.5">Provide an artifact ID to start AI analysis</p>
                </div>
                <button
                  class="w-8 h-8 flex items-center justify-center rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  @click="showTriggerDrawer = false"
                >
                  <XMarkIcon class="w-4 h-4" />
                </button>
              </div>

              <!-- Drawer body -->
              <div class="flex-1 overflow-y-auto px-6 py-5 space-y-4">
                <div>
                  <label class="block text-xs font-medium text-muted mb-1.5">Artifact ID</label>
                  <select
                    v-model="triggerForm.artifactId"
                    class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
                  >
                    <option value="" disabled>Select a package…</option>
                    <option v-for="pkg in packages" :key="pkg.name" :value="pkg.name">
                      {{ pkg.name }}@{{ pkg.version || '?' }}
                    </option>
                  </select>
                  <p v-if="!packages.length" class="text-xs text-muted mt-1">No packages in this repo. Upload a package first.</p>
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted mb-1.5">Manifest text <span class="text-muted font-normal">(optional)</span></label>
                  <textarea
                    v-model="triggerForm.manifestText"
                    rows="5"
                    placeholder="Paste manifest content here…"
                    class="w-full px-3 py-2 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 resize-none font-mono"
                  />
                </div>
              </div>

              <!-- Drawer footer -->
              <div class="px-6 py-4 border-t border-border flex items-center justify-end gap-2 shrink-0">
                <button
                  class="px-4 py-2 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
                  @click="showTriggerDrawer = false"
                >
                  Cancel
                </button>
                <button
                  class="flex items-center gap-2 px-4 py-2 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50"
                  :disabled="triggering || !triggerForm.artifactId.trim()"
                  @click="submitTrigger"
                >
                  <ArrowPathIcon v-if="triggering" class="w-3.5 h-3.5 animate-spin" />
                  <SparklesIcon v-else class="w-3.5 h-3.5" />
                  {{ triggering ? 'Triggering…' : 'Start Analysis' }}
                </button>
              </div>
            </div>
          </Transition>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  ChevronRightIcon,
  ArrowPathIcon,
  MagnifyingGlassIcon,
  SparklesIcon,
  XMarkIcon,
  ExclamationTriangleIcon,
} from '@heroicons/vue/24/outline'
import EmptyState from '../../components/EmptyState.vue'
import FormatBadge from '../../components/FormatBadge.vue'
import { useToastStore } from '../../stores/toast'
import { listAIAnalyses, triggerAIAnalysis } from '../../api/ai'
import { listPackages } from '../../api/repositories'
import type { AIAnalysis } from '../../api/ai'

const toasts = useToastStore()

const repos = ref<{ name: string }[]>([])
const selectedRepo = ref('')
const packages = ref<{ name: string; version: string; format: string }[]>([])
const analyses = ref<AIAnalysis[]>([])
const loadingAnalyses = ref(false)
const triggering = ref(false)
const retriggering = ref<string | null>(null)
const searchQuery = ref('')
const statusFilter = ref('all')
const licenseFilter = ref('all')
const selectedAnalysis = ref<AIAnalysis | null>(null)
const showTriggerDrawer = ref(false)

const triggerForm = ref({
  artifactId: '',
  manifestText: '',
})

const statusOptions = [
  { label: 'All', value: 'all' },
  { label: 'Pending', value: 'pending' },
  { label: 'Processing', value: 'processing' },
  { label: 'Completed', value: 'completed' },
  { label: 'Failed', value: 'failed' },
]

const licenseOptions = [
  { label: 'All', value: 'all' },
  { label: 'Safe', value: 'SAFE' },
  { label: 'Review Required', value: 'REVIEW_REQUIRED' },
  { label: 'Unknown', value: 'UNKNOWN' },
]

const countSafe = computed(() => analyses.value.filter(a => a.license_status === 'SAFE').length)
const countReview = computed(() => analyses.value.filter(a => a.license_status === 'REVIEW_REQUIRED').length)
const countQuarantine = computed(() => analyses.value.filter(a => a.quarantine_recommended).length)

const filteredAnalyses = computed(() => {
  let result = analyses.value
  const q = searchQuery.value.trim().toLowerCase()
  if (q) result = result.filter(a => a.artifact_name.toLowerCase().includes(q))
  if (statusFilter.value !== 'all') result = result.filter(a => a.status === statusFilter.value)
  if (licenseFilter.value !== 'all') result = result.filter(a => a.license_status === licenseFilter.value)
  return result
})

function analysisStatusClass(status: string): string {
  const map: Record<string, string> = {
    pending: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-warning bg-warning-soft/60',
    processing: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-primary bg-primary/10',
    completed: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-success bg-success-soft/60',
    failed: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-danger bg-danger-soft',
  }
  return map[status] || map.pending
}

function licenseStatusClass(status: string): string {
  const map: Record<string, string> = {
    SAFE: 'inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-semibold text-success bg-success-soft/60',
    REVIEW_REQUIRED: 'inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-semibold text-warning bg-warning-soft/60',
    UNKNOWN: 'inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-semibold text-muted bg-surface-hover',
  }
  return map[status] || map.UNKNOWN
}

function licenseStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    SAFE: 'Safe',
    REVIEW_REQUIRED: 'Review Required',
    UNKNOWN: 'Unknown',
  }
  return labels[status] || status
}

function truncate(text: string, length: number): string {
  if (!text) return '—'
  return text.length > length ? text.slice(0, length) + '…' : text
}

function formatDate(ts?: string): string {
  if (!ts) return '—'
  try {
    return new Date(ts).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  } catch {
    return ts
  }
}

function openDetail(analysis: AIAnalysis): void {
  selectedAnalysis.value = analysis
}

function openTriggerDrawer(): void {
  triggerForm.value = { artifactId: '', manifestText: '' }
  showTriggerDrawer.value = true
}

async function loadRepos(): Promise<void> {
  try {
    const res = await fetch('/api/v1/repositories?limit=100', { credentials: 'include' })
    const data = await res.json()
    repos.value = data.repositories || []
  } catch { /* ignore */ }
}

async function loadPackages(): Promise<void> {
  if (!selectedRepo.value) { packages.value = []; return }
  try {
    const res = await listPackages(selectedRepo.value, { limit: 200 })
    packages.value = res.packages || []
  } catch { packages.value = [] }
}

async function loadAnalyses(): Promise<void> {
  if (!selectedRepo.value) return
  loadingAnalyses.value = true
  try {
    const result = await listAIAnalyses(selectedRepo.value, { limit: 200 })
    analyses.value = result.analyses || []
  } catch {
    toasts.error('Failed to load AI analyses')
  } finally {
    loadingAnalyses.value = false
  }
}

async function retrigger(analysis: AIAnalysis): Promise<void> {
  retriggering.value = analysis.id
  try {
    await triggerAIAnalysis(selectedRepo.value, analysis.artifact_id)
    toasts.success('Analysis re-triggered successfully')
    await loadAnalyses()
    // Refresh selected if open
    if (selectedAnalysis.value?.id === analysis.id) {
      selectedAnalysis.value = analyses.value.find(a => a.id === analysis.id) ?? null
    }
  } catch {
    toasts.error('Failed to re-trigger analysis')
  } finally {
    retriggering.value = null
  }
}

async function submitTrigger(): Promise<void> {
  if (!triggerForm.value.artifactId.trim()) return
  triggering.value = true
  try {
    await triggerAIAnalysis(
      selectedRepo.value,
      triggerForm.value.artifactId.trim(),
      triggerForm.value.manifestText || undefined,
    )
    toasts.success('Analysis triggered successfully')
    showTriggerDrawer.value = false
    triggerForm.value = { artifactId: '', manifestText: '' }
    await loadAnalyses()
  } catch {
    toasts.error('Failed to trigger analysis')
  } finally {
    triggering.value = false
  }
}

onMounted(async () => { await loadRepos() })

import { watch } from 'vue'
watch(selectedRepo, () => { loadPackages(); loadAnalyses() })
</script>
