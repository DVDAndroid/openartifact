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
          <span class="text-foreground/70">Vulnerabilities</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">Vulnerability Scanning</h1>
        <p class="mt-1 text-sm text-muted">Detect known CVEs across your artifacts using the OSV database.</p>
      </div>
      <div class="flex items-center gap-3 flex-wrap">
        <!-- Scanner selector -->
        <div class="flex items-center gap-1 bg-surface-hover/60 rounded-lg p-0.5 border border-border">
          <button
            v-for="s in scanners"
            :key="s"
            class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
            :class="activeScanner === s ? 'bg-surface text-foreground shadow-sm' : 'text-muted hover:text-foreground'"
            @click="activeScanner = s"
          >{{ s }}</button>
        </div>
        <button
          class="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
          @click="showDrawer = true"
        >
          <ShieldCheckIcon class="w-4 h-4" />
          Scan repository
        </button>
      </div>
    </div>

    <!-- Repo selector + refresh -->
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
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Total Scans</div>
        <div class="text-2xl font-bold mt-1 tabular-nums">{{ scans.length }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Critical</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-danger">{{ totalCritical }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">High</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-warning">{{ totalHigh }}</div>
      </div>
      <div class="rounded-xl border border-border bg-surface p-4">
        <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Medium</div>
        <div class="text-2xl font-bold mt-1 tabular-nums text-[#f59e0b]">{{ totalMedium }}</div>
      </div>
    </div>

    <!-- Severity distribution bar -->
    <div class="bg-surface rounded-xl border border-border p-5">
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-sm font-semibold">Severity Distribution</h3>
        <span class="text-xs text-muted font-mono">{{ totalVulns }} total vulnerabilities</span>
      </div>
      <div class="flex h-2.5 rounded-full overflow-hidden bg-surface-hover">
        <div v-if="totalVulns > 0" class="h-full bg-danger transition-all" :style="{ width: `${(totalCritical / totalVulns) * 100}%` }" />
        <div v-if="totalVulns > 0" class="h-full bg-warning transition-all" :style="{ width: `${(totalHigh / totalVulns) * 100}%` }" />
        <div v-if="totalVulns > 0" class="h-full bg-[#f59e0b] transition-all" :style="{ width: `${(totalMedium / totalVulns) * 100}%` }" />
        <div v-if="totalVulns > 0" class="h-full bg-muted/40 transition-all" :style="{ width: `${(totalLow / totalVulns) * 100}%` }" />
      </div>
      <div class="flex items-center gap-4 mt-3 text-xs text-muted flex-wrap">
        <span class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-danger" /> Critical: {{ totalCritical }}</span>
        <span class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-warning" /> High: {{ totalHigh }}</span>
        <span class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-[#f59e0b]" /> Medium: {{ totalMedium }}</span>
        <span class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-muted/40" /> Low: {{ totalLow }}</span>
      </div>
    </div>

    <!-- Filter toolbar -->
    <div class="flex items-center gap-3 flex-wrap">
      <div class="relative flex-1 max-w-sm">
        <MagnifyingGlassIcon class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
        <input
          v-model="searchQuery"
          type="search"
          placeholder="Search packages…"
          class="w-full pl-9 pr-3 h-9 text-sm rounded-lg bg-surface-hover/40 border border-border text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
        />
      </div>
      <div class="flex items-center gap-1 bg-surface-hover/60 rounded-lg p-0.5">
        <button
          v-for="s in severityOptions"
          :key="s.value"
          class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
          :class="severityFilter === s.value ? 'bg-surface text-foreground shadow-sm' : 'text-muted hover:text-foreground'"
          @click="severityFilter = s.value"
        >{{ s.label }}</button>
      </div>
      <div class="flex items-center gap-1 bg-surface-hover/60 rounded-lg p-0.5">
        <button
          v-for="s in statusOptions"
          :key="s.value"
          class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
          :class="statusFilter === s.value ? 'bg-surface text-foreground shadow-sm' : 'text-muted hover:text-foreground'"
          @click="statusFilter = s.value"
        >{{ s.label }}</button>
      </div>
    </div>

    <!-- Scans table -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Scan Results</h3>
        <span class="text-[11px] font-mono text-muted">{{ filteredScans.length }} scans</span>
      </div>

      <div v-if="loadingScans" class="py-12 flex flex-col items-center gap-3 text-muted">
        <ArrowPathIcon class="w-6 h-6 animate-spin" />
        <span class="text-sm">Loading scans…</span>
      </div>

      <EmptyState
        v-else-if="!selectedRepo"
        title="Select a repository"
        description="Choose a repository above to view its vulnerability scans."
      />

      <EmptyState
        v-else-if="filteredScans.length === 0"
        title="No scans found"
        description="Trigger a scan using the button above to detect vulnerabilities."
      />

      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="border-b border-border bg-surface-hover/40">
            <tr>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Package</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Version</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Ecosystem</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Scanner</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Vulns</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Status</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Scanned</th>
              <th class="text-left px-4 py-2.5 text-muted font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="scan in filteredScans"
              :key="scan.id"
              class="hover:bg-surface-hover/40 transition-colors border-b border-border last:border-0"
            >
              <td class="px-4 py-3 font-mono text-[13px] font-medium text-foreground">{{ scan.package_name }}</td>
              <td class="px-4 py-3 font-mono text-[13px] text-muted">{{ scan.package_version }}</td>
              <td class="px-4 py-3 text-sm text-muted">{{ scan.ecosystem }}</td>
              <td class="px-4 py-3">
                <span class="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-semibold border border-border text-muted uppercase tracking-wide">{{ scan.scanner || 'osv' }}</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1.5 text-xs font-mono">
                  <span v-if="scan.critical_count > 0" class="px-1.5 py-0.5 rounded bg-danger-soft text-danger font-semibold">{{ scan.critical_count }}C</span>
                  <span v-if="scan.high_count > 0" class="px-1.5 py-0.5 rounded bg-warning-soft text-warning font-semibold">{{ scan.high_count }}H</span>
                  <span v-if="scan.medium_count > 0" class="px-1.5 py-0.5 rounded bg-[#f59e0b]/10 text-[#f59e0b] font-semibold">{{ scan.medium_count }}M</span>
                  <span v-if="scan.vuln_count === 0 && scan.scan_status === 'completed'" class="text-success text-xs">Clean</span>
                  <span v-if="scan.scan_status === 'scanning'" class="text-muted text-xs">—</span>
                </div>
              </td>
              <td class="px-4 py-3">
                <span :class="scanStatusClass(scan.scan_status)">
                  <ArrowPathIcon v-if="scan.scan_status === 'scanning'" class="w-3 h-3 animate-spin" />
                  {{ scan.scan_status }}
                </span>
              </td>
              <td class="px-4 py-3 text-xs text-muted font-mono">{{ formatDate(scan.scanned_at || scan.created_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1.5">
                  <button
                    class="px-2 py-1 rounded-md text-[11px] font-medium border border-border text-muted hover:bg-surface-hover hover:text-foreground transition-colors"
                    :class="{ 'border-danger/30 text-danger bg-danger-soft/40': scan.quarantined }"
                    @click="toggleQuarantine(scan)"
                  >
                    {{ scan.quarantined ? 'Unquarantine' : 'Quarantine' }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Scan drawer -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-200"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div v-if="showDrawer" class="fixed inset-0 z-50 flex justify-end" @click.self="showDrawer = false">
          <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="showDrawer = false" />
          <Transition
            enter-active-class="transition-transform duration-300 ease-out"
            enter-from-class="translate-x-full"
            enter-to-class="translate-x-0"
            leave-active-class="transition-transform duration-200 ease-in"
            leave-from-class="translate-x-0"
            leave-to-class="translate-x-full"
          >
            <div v-if="showDrawer" class="relative w-full max-w-md h-full bg-surface border-l border-border shadow-2xl flex flex-col">
              <!-- Drawer header -->
              <div class="flex items-center justify-between px-6 py-4 border-b border-border">
                <div>
                  <h2 class="text-base font-semibold">New Vulnerability Scan</h2>
                  <p class="text-xs text-muted mt-0.5">Powered by {{ activeScanner }}</p>
                </div>
                <button class="w-8 h-8 flex items-center justify-center rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors" @click="showDrawer = false">
                  <XMarkIcon class="w-4 h-4" />
                </button>
              </div>

              <!-- Drawer body -->
              <div class="flex-1 overflow-y-auto px-6 py-5 space-y-4">
                <div>
                  <label class="block text-xs font-medium text-muted mb-1.5">Repository</label>
                  <select v-model="form.repoName" class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50">
                    <option value="">Select repository…</option>
                    <option v-for="r in repos" :key="r.name" :value="r.name">{{ r.name }}</option>
                  </select>
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted mb-1.5">Package name</label>
                  <input v-model="form.packageName" type="text" placeholder="e.g. lodash" class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted mb-1.5">Version</label>
                  <input v-model="form.packageVersion" type="text" placeholder="e.g. 4.17.15" class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-muted mb-1.5">Ecosystem</label>
                  <select v-model="form.ecosystem" class="w-full h-9 px-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50">
                    <option value="">Select ecosystem…</option>
                    <option v-for="e in ecosystems" :key="e" :value="e">{{ e }}</option>
                  </select>
                </div>
              </div>

              <!-- Drawer footer -->
              <div class="px-6 py-4 border-t border-border flex items-center justify-end gap-2">
                <button class="px-4 py-2 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors" @click="showDrawer = false">Cancel</button>
                <button
                  class="flex items-center gap-2 px-4 py-2 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50"
                  :disabled="scanning || !form.repoName || !form.packageName || !form.packageVersion || !form.ecosystem"
                  @click="startScan"
                >
                  <ArrowPathIcon v-if="scanning" class="w-3.5 h-3.5 animate-spin" />
                  <ShieldCheckIcon v-else class="w-3.5 h-3.5" />
                  {{ scanning ? 'Starting…' : 'Start Scan' }}
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
  ChevronRightIcon, ArrowPathIcon, MagnifyingGlassIcon,
  ShieldCheckIcon, XMarkIcon,
} from '@heroicons/vue/24/outline'
import EmptyState from '../../components/EmptyState.vue'
import { useToastStore } from '../../stores/toast'

const toasts = useToastStore()

interface VulnScan {
  id: string
  package_name: string
  package_version: string
  ecosystem: string
  scanner: string
  scan_status: string
  vuln_count: number
  critical_count: number
  high_count: number
  medium_count: number
  low_count: number
  quarantined: boolean
  created_at: string
  scanned_at?: string
  scan_error?: string
}

const repos = ref<any[]>([])
const selectedRepo = ref('')
const scans = ref<VulnScan[]>([])
const loadingScans = ref(false)
const showDrawer = ref(false)
const scanning = ref(false)
const searchQuery = ref('')
const severityFilter = ref('all')
const statusFilter = ref('all')
const activeScanner = ref('OSV-Scanner')

const scanners = ['Grype', 'Trivy', 'OSV-Scanner']
const ecosystems = ['npm', 'PyPI', 'Go', 'Maven', 'NuGet', 'RubyGems', 'Cargo', 'Hex', 'Pub', 'Linux']

const form = ref({
  repoName: '',
  packageName: '',
  packageVersion: '',
  ecosystem: '',
})

const severityOptions = [
  { label: 'All', value: 'all' },
  { label: 'Critical', value: 'critical' },
  { label: 'High', value: 'high' },
  { label: 'Medium', value: 'medium' },
  { label: 'Low', value: 'low' },
]

const statusOptions = [
  { label: 'All', value: 'all' },
  { label: 'Scanning', value: 'scanning' },
  { label: 'Completed', value: 'completed' },
  { label: 'Failed', value: 'failed' },
]

const totalCritical = computed(() => scans.value.reduce((s, r) => s + (r.critical_count || 0), 0))
const totalHigh = computed(() => scans.value.reduce((s, r) => s + (r.high_count || 0), 0))
const totalMedium = computed(() => scans.value.reduce((s, r) => s + (r.medium_count || 0), 0))
const totalLow = computed(() => scans.value.reduce((s, r) => s + (r.low_count || 0), 0))
const totalVulns = computed(() => totalCritical.value + totalHigh.value + totalMedium.value + totalLow.value)

const filteredScans = computed(() => {
  let result = scans.value
  const q = searchQuery.value.trim().toLowerCase()
  if (q) result = result.filter(s => s.package_name.toLowerCase().includes(q))
  if (statusFilter.value !== 'all') result = result.filter(s => s.scan_status === statusFilter.value)
  if (severityFilter.value !== 'all') {
    result = result.filter(s => {
      if (severityFilter.value === 'critical') return s.critical_count > 0
      if (severityFilter.value === 'high') return s.high_count > 0
      if (severityFilter.value === 'medium') return s.medium_count > 0
      if (severityFilter.value === 'low') return s.low_count > 0
      return true
    })
  }
  return result
})

function scanStatusClass(status: string): string {
  const map: Record<string, string> = {
    scanning: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-warning bg-warning-soft/60',
    completed: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-success bg-success-soft/60',
    failed: 'inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold text-danger bg-danger-soft/60',
  }
  return map[status] || map.completed
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
    const res = await fetch(`/api/v1/repositories/${encodeURIComponent(selectedRepo.value)}/vuln/scans`, { credentials: 'include' })
    const data = await res.json()
    scans.value = data.scans || []
  } catch {
    toasts.error('Failed to load scans')
  } finally {
    loadingScans.value = false
  }
}

async function startScan(): Promise<void> {
  scanning.value = true
  try {
    const res = await fetch(`/api/v1/repositories/${encodeURIComponent(form.value.repoName)}/vuln/scan`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        artifact_id: 'manual-scan',
        package_name: form.value.packageName,
        package_version: form.value.packageVersion,
        ecosystem: form.value.ecosystem,
        scanner: { 'Grype': 'grype', 'Trivy': 'trivy', 'OSV-Scanner': 'osv' }[activeScanner.value] ?? 'osv',
      }),
    })
    if (!res.ok) throw new Error('Scan failed')
    toasts.success('Scan started successfully')
    showDrawer.value = false
    selectedRepo.value = form.value.repoName
    form.value = { repoName: '', packageName: '', packageVersion: '', ecosystem: '' }
    await loadScans()
  } catch {
    toasts.error('Failed to start scan')
  } finally {
    scanning.value = false
  }
}

async function toggleQuarantine(scan: VulnScan): Promise<void> {
  try {
    await fetch(`/api/v1/repositories/${encodeURIComponent(selectedRepo.value)}/vuln/scans/${scan.id}/quarantine`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ quarantined: !scan.quarantined }),
    })
    scan.quarantined = !scan.quarantined
    toasts.success(scan.quarantined ? 'Artifact quarantined' : 'Quarantine removed')
  } catch {
    toasts.error('Failed to update quarantine status')
  }
}

onMounted(loadRepos)
</script>
