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
          <span class="text-foreground/70">{{ t('nav.gpg') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.gpg.title') }}</h1>
        <p class="mt-1 text-sm text-muted">{{ t('admin.gpg.description') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors font-medium"
          @click="importOpen = true"
        >
          {{ t('admin.gpg.import') }}
        </button>
        <button
          type="button"
          class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
          @click="generateOpen = true"
        >
          {{ t('admin.gpg.generate') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="text-muted">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="keys.length === 0" :title="t('common.empty')" />
    <DataTable v-else :columns="columns" :rows="keys as Record<string, unknown>[]" row-key="fingerprint">
      <template #cell-fingerprint="{ row }">
        <span class="font-mono text-xs text-foreground">{{ row.fingerprint }}</span>
      </template>
      <template #cell-actions="{ row }">
        <button class="text-xs text-danger hover:opacity-80" @click="revoke(String(row.fingerprint))">
          {{ t('admin.gpg.revoke') }}
        </button>
      </template>
    </DataTable>

    <!-- Import Drawer -->
    <Teleport to="body">
      <div v-if="importOpen" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="importOpen = false" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">GPG Key</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">{{ t('admin.gpg.import') }}</h2>
            </div>
            <button @click="importOpen = false" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Armored key</label>
              <textarea
                v-model="importForm.armored"
                rows="10"
                required
                class="w-full px-3 py-2 border border-border rounded-lg text-xs font-mono bg-surface-hover/40 text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Passphrase (optional)</label>
              <input
                v-model="importForm.passphrase"
                type="password"
                placeholder="Optional passphrase"
                class="w-full px-3 py-2 border border-border rounded-lg text-sm bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button @click="importOpen = false" class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors">
              {{ t('common.cancel') }}
            </button>
            <button @click="submitImport" class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium">
              {{ t('admin.gpg.import') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Generate Drawer -->
    <Teleport to="body">
      <div v-if="generateOpen" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="generateOpen = false" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">GPG Key</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">{{ t('admin.gpg.generate') }}</h2>
            </div>
            <button @click="generateOpen = false" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Name</label>
              <input v-model="genForm.name" required type="text" placeholder="e.g. John Doe" class="w-full px-3 py-2 border border-border rounded-lg text-sm bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50" />
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Email</label>
              <input v-model="genForm.email" required type="email" placeholder="john@example.com" class="w-full px-3 py-2 border border-border rounded-lg text-sm bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50" />
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Comment</label>
              <input v-model="genForm.comment" type="text" placeholder="Optional note" class="w-full px-3 py-2 border border-border rounded-lg text-sm bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50" />
            </div>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button @click="generateOpen = false" class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors">
              {{ t('common.cancel') }}
            </button>
            <button @click="submitGenerate" class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium">
              {{ t('admin.gpg.generate') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, Teleport } from 'vue'
import { ChevronRightIcon } from '@heroicons/vue/20/solid'
import { XMarkIcon } from '@heroicons/vue/24/outline'
import { listGpgKeys, importGpgKey, generateGpgKey, revokeGpgKey, type GpgKey } from '../../api/gpg'
import DataTable from '../../components/DataTable.vue'
import type { Column } from '../../components/DataTable.vue'
import EmptyState from '../../components/EmptyState.vue'
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'

const keys = ref<GpgKey[]>([])
const loading = ref(false)
const toasts = useToastStore()

const importOpen = ref(false)
const generateOpen = ref(false)
const importForm = ref({ armored: '', passphrase: '' })
const genForm = ref({ name: '', email: '', comment: '' })

const columns: Column[] = [
  { key: 'fingerprint', label: t('admin.gpg.column.fingerprint') },
  { key: 'uid', label: t('admin.gpg.column.uid') },
  { key: 'created_at', label: t('admin.gpg.column.created') },
  { key: 'actions', label: t('common.actions') },
]

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listGpgKeys()
    keys.value = res.items
  } finally {
    loading.value = false
  }
}

async function submitImport(): Promise<void> {
  try {
    await importGpgKey({ armored: importForm.value.armored, passphrase: importForm.value.passphrase || undefined })
    toasts.success('Imported')
    importOpen.value = false
    importForm.value = { armored: '', passphrase: '' }
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

async function submitGenerate(): Promise<void> {
  try {
    await generateGpgKey({ ...genForm.value })
    toasts.success('Generated')
    generateOpen.value = false
    genForm.value = { name: '', email: '', comment: '' }
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

async function revoke(fingerprint: string): Promise<void> {
  if (!window.confirm(t('admin.gpg.revoke') + '?')) return
  try {
    await revokeGpgKey(fingerprint)
    toasts.success('Revoked')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

onMounted(load)
</script>
