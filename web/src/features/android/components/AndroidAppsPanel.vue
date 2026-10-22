<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- The Apps tab of an android repository: searchable list of packages with upload. -->
<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <input
        v-model="search"
        type="search"
        class="h-9 w-full sm:w-64 rounded-lg border border-border-strong bg-surface px-3 text-sm text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary"
        :placeholder="t('android.apps.search')"
        :aria-label="t('android.apps.search')"
      />
      <select v-model="origin" class="select" :aria-label="t('android.apps.filterOrigin')">
        <option value="">{{ t('android.apps.allOrigins') }}</option>
        <option v-for="o in ORIGINS" :key="o" :value="o">{{ originLabel(o) }}</option>
      </select>
      <select v-model="sort" class="select" :aria-label="t('android.apps.sort')">
        <option value="name">{{ t('android.apps.sort.name') }}</option>
        <option value="updated">{{ t('android.apps.sort.updated') }}</option>
        <option value="versions">{{ t('android.apps.sort.versions') }}</option>
        <option value="size">{{ t('android.apps.sort.size') }}</option>
      </select>
      <AppButton v-if="canWrite" class="ml-auto" variant="secondary" :aria-expanded="showUpload" @click="showUpload = !showUpload">
        <ArrowUpTrayIcon class="w-4 h-4 text-muted" aria-hidden="true" />
        {{ t('android.apps.upload') }}
      </AppButton>
    </div>

    <div v-if="tagCounts.length" class="flex flex-wrap items-center gap-1.5" role="group" :aria-label="t('android.apps.filterTags')">
      <button
        v-for="tc in tagCounts"
        :key="tc.tag"
        type="button"
        class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-mono transition-colors"
        :class="activeTags.includes(tc.tag) ? 'bg-primary text-primary-contrast' : 'bg-surface-hover text-foreground hover:bg-border'"
        :aria-pressed="activeTags.includes(tc.tag)"
        @click="toggleTag(tc.tag)"
      >
        {{ tc.tag }}
        <span class="opacity-70">{{ tc.apps }}</span>
      </button>
    </div>

    <AndroidUploadZone v-if="canWrite && showUpload" :repo="repo" @uploaded="onUploaded" />

    <p v-if="error" class="text-sm text-danger">{{ error }}</p>
    <div v-else-if="loading && apps.length === 0" class="text-sm text-muted">{{ t('common.loading') }}</div>

    <template v-else-if="apps.length === 0">
      <EmptyState
        v-if="hasFilter"
        :title="t('android.apps.noMatch')"
        :description="t('android.apps.noMatchHint')"
      />
      <div v-else class="space-y-4">
        <EmptyState :title="t('android.apps.emptyTitle')" :description="t('android.apps.emptyHint')" />
        <AndroidUploadZone v-if="canWrite && !showUpload" :repo="repo" @uploaded="onUploaded" />
      </div>
    </template>

    <div v-else class="overflow-x-auto rounded-xl border border-border bg-surface">
      <table class="w-full">
        <thead>
          <tr class="border-b border-border bg-surface-hover text-xs font-semibold text-muted">
            <th class="text-left px-4 py-2.5">{{ t('android.col.app') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.col.latest') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.col.origin') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.col.signer') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.col.tags') }}</th>
            <th class="text-right px-4 py-2.5">{{ t('android.col.versions') }}</th>
            <th class="text-right px-4 py-2.5">{{ t('android.col.size') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.col.updated') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="app in apps" :key="app.id" class="border-t border-border first:border-t-0 hover:bg-surface-hover transition-colors">
            <td class="px-4 py-2.5">
              <router-link :to="appLink(app.package_name)" class="flex items-center gap-3 min-w-0 group">
                <AppIcon :src="app.icon_url" :label="app.label || app.package_name" :size="36" />
                <span class="min-w-0">
                  <span class="block text-sm font-medium text-foreground group-hover:text-primary truncate">{{ app.label || app.package_name }}</span>
                  <span class="block text-xs font-mono text-muted truncate">{{ app.package_name }}</span>
                </span>
              </router-link>
            </td>
            <td class="px-4 py-2.5 text-xs">
              <span class="font-mono text-foreground">{{ app.latest_version_name || '—' }}</span>
              <span class="block font-mono text-muted">{{ app.latest_version_code }}</span>
            </td>
            <td class="px-4 py-2.5">
              <div class="flex flex-wrap gap-1">
                <StatusBadge v-for="o in app.origins" :key="o" :label="originLabel(o)" :variant="originVariant(o)" />
              </div>
            </td>
            <td class="px-4 py-2.5">
              <SignerBadge
                v-if="app.latest_signer_sha256"
                :sha256="app.latest_signer_sha256"
                :trust="trustOf(app.latest_signer_sha256)?.trust"
                :known="!!trustOf(app.latest_signer_sha256)"
                :copyable="false"
              />
              <span v-if="app.signers.length > 1" class="ml-1 text-xs text-warning">{{ t('android.apps.keys').replace('{n}', String(app.signers.length)) }}</span>
            </td>
            <td class="px-4 py-2.5">
              <TagEditor v-if="app.tags.length" :model-value="app.tags" />
              <span v-else class="text-xs text-muted">—</span>
            </td>
            <td class="px-4 py-2.5 text-right text-xs font-mono tabular-nums text-muted">{{ app.version_count }}</td>
            <td class="px-4 py-2.5 text-right text-xs font-mono tabular-nums text-muted">{{ formatBytes(app.total_size_bytes) }}</td>
            <td class="px-4 py-2.5 text-xs text-muted">{{ formatDate(app.latest_created_at || app.updated_at) }}</td>
          </tr>
        </tbody>
      </table>
      <Pagination v-if="total > LIMIT" :total="total" :page="page" :limit="LIMIT" @change="(p: number) => (page = p)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ArrowUpTrayIcon } from '@heroicons/vue/24/outline'
import AppButton from '../../../components/AppButton.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Pagination from '../../../components/Pagination.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { formatDate } from '../../../lib/datetime'
import { t } from '../../../i18n/en'
import { listApps, listSigners, listTags } from '../api'
import { formatBytes, originVariant } from '../lib'
import { ORIGINS, type AndroidAppSummary, type AndroidSigner, type AppSort, type Origin, type TagCount } from '../types'
import { originLabel } from '../labels'
import AndroidUploadZone from './AndroidUploadZone.vue'
import AppIcon from './AppIcon.vue'
import SignerBadge from './SignerBadge.vue'
import TagEditor from './TagEditor.vue'

const props = defineProps<{ repo: string; canWrite: boolean }>()
const emit = defineEmits<{ loaded: [total: number] }>()

const LIMIT = 25

const apps = ref<AndroidAppSummary[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const error = ref('')
const search = ref('')
const origin = ref<Origin | ''>('')
const sort = ref<AppSort>('name')
const activeTags = ref<string[]>([])
const tagCounts = ref<TagCount[]>([])
const signers = ref<AndroidSigner[]>([])
const showUpload = ref(false)

const hasFilter = computed(() => !!search.value || !!origin.value || activeTags.value.length > 0)

function appLink(pkg: string): string {
  return `/repos/${encodeURIComponent(props.repo)}/packages/android/${encodeURIComponent(pkg)}`
}

function trustOf(sha: string): AndroidSigner | undefined {
  return signers.value.find((s) => s.sha256 === sha)
}

function toggleTag(tag: string): void {
  activeTags.value = activeTags.value.includes(tag)
    ? activeTags.value.filter((x) => x !== tag)
    : [...activeTags.value, tag]
}

let generation = 0

async function load(): Promise<void> {
  const mine = ++generation
  loading.value = true
  error.value = ''
  try {
    const res = await listApps(props.repo, {
      q: search.value.trim(), tags: activeTags.value, origin: origin.value, sort: sort.value,
      page: page.value, limit: LIMIT,
    })
    if (mine !== generation) return
    apps.value = res.items ?? []
    total.value = res.total
    // Total of the unfiltered repository drives the tab counter.
    if (!hasFilter.value) emit('loaded', res.total)
  } catch (err) {
    if (mine !== generation) return
    error.value = err instanceof Error ? err.message : t('android.apps.loadFailed')
  } finally {
    if (mine === generation) loading.value = false
  }
}

async function loadSide(): Promise<void> {
  // Badges and the tag cloud are decoration: a failure leaves them empty.
  const [tags, keys] = await Promise.allSettled([listTags(props.repo), listSigners(props.repo)])
  // The filter matches app tags, so a tag only carried by versions would filter to nothing.
  tagCounts.value = tags.status === 'fulfilled' ? (tags.value ?? []).filter((x) => x.apps > 0) : []
  signers.value = keys.status === 'fulfilled' ? (keys.value ?? []) : []
}

function onUploaded(): void {
  void load()
  void loadSide()
}

let timer: ReturnType<typeof setTimeout> | undefined
watch([search, origin, sort, activeTags], () => {
  page.value = 1
  clearTimeout(timer)
  timer = setTimeout(() => void load(), 250)
})
watch(page, () => void load())
watch(() => props.repo, () => { page.value = 1; void load(); void loadSide() }, { immediate: true })
onBeforeUnmount(() => clearTimeout(timer))
</script>

<style scoped>
.select {
  height: 2.25rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--color-border-strong));
  background: rgb(var(--color-surface));
  padding: 0 0.5rem;
  font-size: 0.875rem;
  color: rgb(var(--color-foreground));
}
</style>
