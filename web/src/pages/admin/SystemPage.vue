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
          <span class="text-foreground/70">{{ t('nav.system') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.system.title') }}</h1>
        <p class="mt-1 text-sm text-muted">Server health and configuration overview</p>
      </div>
    </div>

    <div v-if="loading" class="text-muted">Loading...</div>
    <UnavailableStub v-else-if="!sysInfo" />
    <div v-else class="space-y-5">
      <!-- Stat Row: Version · Go Version · OS/Arch · Uptime -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
        <!-- Version -->
        <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
          <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft shrink-0">
            <CodeBracketIcon class="w-4 h-4" />
          </span>
          <div class="min-w-0">
            <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Version</div>
            <div class="text-lg font-bold font-mono truncate">{{ sysInfo.version || '—' }}</div>
          </div>
        </div>

        <!-- Go Version -->
        <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
          <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft shrink-0">
            <CodeBracketIcon class="w-4 h-4" />
          </span>
          <div class="min-w-0">
            <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Go Version</div>
            <div class="text-lg font-bold font-mono truncate">{{ sysInfo.go_version || '—' }}</div>
          </div>
        </div>

        <!-- OS / Arch -->
        <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
          <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft shrink-0">
            <ServerIcon class="w-4 h-4" />
          </span>
          <div class="min-w-0">
            <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">OS / Arch</div>
            <div class="text-lg font-bold font-mono truncate">{{ osArch }}</div>
          </div>
        </div>

        <!-- Uptime -->
        <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
          <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft shrink-0">
            <ClockIcon class="w-4 h-4" />
          </span>
          <div class="min-w-0">
            <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Uptime</div>
            <div class="text-lg font-bold font-mono truncate">{{ sysInfo.uptime || '—' }}</div>
          </div>
        </div>
      </div>

      <!-- Info Cards -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <!-- Build Card -->
        <div class="bg-surface rounded-xl border border-border p-5">
          <h3 class="text-sm font-semibold mb-3">Build</h3>
          <div class="space-y-0">
            <div class="flex items-center justify-between py-2 border-b border-border">
              <span class="text-sm text-muted">Version</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.version || '—' }}</span>
            </div>
            <div class="flex items-center justify-between py-2 border-b border-border">
              <span class="text-sm text-muted">Commit</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.commit || '—' }}</span>
            </div>
            <div class="flex items-center justify-between py-2 border-b border-border">
              <span class="text-sm text-muted">Build Time</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.build_time || '—' }}</span>
            </div>
            <div class="flex items-center justify-between py-2 border-b border-border">
              <span class="text-sm text-muted">Go Version</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.go_version || '—' }}</span>
            </div>
            <div class="flex items-center justify-between py-2 border-b border-border">
              <span class="text-sm text-muted">OS</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.os || '—' }}</span>
            </div>
            <div class="flex items-center justify-between py-2">
              <span class="text-sm text-muted">Arch</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.arch || '—' }}</span>
            </div>
          </div>
        </div>

        <!-- Database Card -->
        <div class="bg-surface rounded-xl border border-border p-5">
          <h3 class="text-sm font-semibold mb-3">Database</h3>
          <div class="space-y-0">
            <div class="flex items-center justify-between py-2">
              <span class="text-sm text-muted">Status</span>
              <div class="flex items-center gap-2">
                <span
                  :class="[
                    'w-2 h-2 rounded-full shrink-0',
                    dbHealthy ? 'bg-success' : 'bg-danger',
                  ]"
                />
                <span
                  :class="[
                    'text-sm font-mono font-medium',
                    dbHealthy ? 'text-success' : 'text-danger',
                  ]"
                >
                  {{ sysInfo.db_status || '—' }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- Storage Card -->
        <div class="bg-surface rounded-xl border border-border p-5">
          <h3 class="text-sm font-semibold mb-3">Storage</h3>
          <div class="space-y-0">
            <div class="flex items-center justify-between py-2 border-b border-border">
              <span class="text-sm text-muted">Kind</span>
              <span class="text-sm font-mono text-foreground">{{ sysInfo.storage_kind || '—' }}</span>
            </div>
            <div class="flex items-center justify-between py-2">
              <span class="text-sm text-muted">Status</span>
              <div class="flex items-center gap-2">
                <span
                  :class="[
                    'w-2 h-2 rounded-full shrink-0',
                    storageHealthy ? 'bg-success' : 'bg-danger',
                  ]"
                />
                <span
                  :class="[
                    'text-sm font-mono font-medium',
                    storageHealthy ? 'text-success' : 'text-danger',
                  ]"
                >
                  {{ sysInfo.storage_status || '—' }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getSystemInfo } from '../../api/admin'
import UnavailableStub from '../../components/UnavailableStub.vue'
import { t } from '../../i18n/en'
import {
  ChevronRightIcon,
  CodeBracketIcon,
  ClockIcon,
  ServerIcon,
} from '@heroicons/vue/24/outline'

// Actual API shape from /api/v1/system/info plus upcoming enhanced fields
type SystemInfo = {
  version?: string
  commit?: string
  build_time?: string
  go_version?: string
  os?: string
  arch?: string
  // Enhanced fields — present after backend upgrade
  uptime?: string
  db_status?: string
  storage_kind?: string
  storage_status?: string
}

const sysInfo = ref<SystemInfo | null>(null)
const loading = ref(false)

const osArch = computed(() => {
  const { os, arch } = sysInfo.value ?? {}
  if (os && arch) return `${os}/${arch}`
  return os || arch || '—'
})

const dbHealthy = computed(() => sysInfo.value?.db_status === 'ok')
const storageHealthy = computed(() => sysInfo.value?.storage_status === 'ok')

onMounted(async () => {
  loading.value = true
  try {
    sysInfo.value = await getSystemInfo() as SystemInfo | null
  } finally {
    loading.value = false
  }
})
</script>
