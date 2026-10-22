<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-4">
    <section class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <h3 class="text-sm font-semibold text-foreground">{{ t('android.prov.title') }}</h3>
      <dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1.5 text-xs">
        <dt class="text-muted">{{ t('android.v.origin') }}</dt>
        <dd><StatusBadge :label="originLabel(v.origin)" :variant="originVariant(v.origin)" /></dd>
        <dt class="text-muted">{{ t('android.prov.parent') }}</dt>
        <dd class="text-foreground">
          <router-link v-if="detail.parent" :to="{ query: { ...route.query, v: detail.parent.id, tab: 'provenance' } }" class="text-primary hover:underline">
            {{ detail.parent.version_name }} ({{ detail.parent.version_code }}) · {{ originLabel(detail.parent.origin) }}
          </router-link>
          <span v-else class="text-muted">{{ t('android.prov.noParent') }}</span>
        </dd>
        <dt class="text-muted">{{ t('android.prov.originalSha') }}</dt>
        <dd class="font-mono text-foreground break-all">{{ v.original_sha256 || '—' }}</dd>
        <dt class="text-muted">{{ t('android.v.sourceUrl') }}</dt>
        <dd class="min-w-0 break-all text-foreground">
          <a v-if="v.source_url && /^https?:\/\//i.test(v.source_url)" :href="v.source_url" target="_blank" rel="noopener noreferrer" class="text-primary hover:underline">{{ v.source_url }}</a>
          <template v-else>{{ v.source_url || '—' }}</template>
        </dd>
        <dt class="text-muted">{{ t('android.v.contentDigest') }}</dt>
        <dd class="font-mono text-foreground break-all">{{ v.content_digest || '—' }}</dd>
      </dl>
      <div v-if="v.patch_info && Object.keys(v.patch_info).length">
        <h4 class="text-xs font-semibold text-muted mb-1">{{ t('android.prov.patchInfo') }}</h4>
        <pre class="max-h-72 overflow-auto rounded-lg bg-surface-hover p-3 text-xs font-mono text-foreground">{{ JSON.stringify(v.patch_info, null, 2) }}</pre>
      </div>
    </section>

    <section v-if="canWrite" class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <h3 class="text-sm font-semibold text-foreground">{{ t('android.prov.edit') }}</h3>
      <div class="grid gap-3 sm:grid-cols-2">
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.v.origin') }}
          <select v-model="origin" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground">
            <option v-for="o in ORIGINS" :key="o" :value="o">{{ originLabel(o) }}</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.prov.parent') }}
          <select v-model="parentId" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground">
            <option value="">{{ t('android.prov.noParent') }}</option>
            <option v-for="s in candidates" :key="s.id" :value="s.id">{{ s.version_name }} ({{ s.version_code }}) · {{ originLabel(s.origin) }} · {{ shortFingerprint(s.signer_sha256) }}</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground sm:col-span-2">
          {{ t('android.v.sourceUrl') }}
          <input v-model="sourceUrl" type="url" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground" placeholder="https://" />
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.v.channel') }}
          <input v-model="channel" type="text" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground" placeholder="stable" />
        </label>
        <label class="flex items-center gap-2 text-xs font-medium text-foreground self-end pb-2">
          <input v-model="pinned" type="checkbox" class="accent-[rgb(var(--color-primary))]" />
          {{ t('android.prov.pinned') }}
        </label>
      </div>
      <div class="flex justify-end">
        <AppButton size="sm" :loading="busy" @click="save">{{ t('common.save') }}</AppButton>
      </div>
    </section>

    <section class="rounded-xl border border-border bg-surface">
      <header class="px-4 py-2.5 border-b border-border">
        <h3 class="text-sm font-semibold text-foreground">{{ t('android.prov.files') }} <span class="text-muted font-normal">({{ detail.files.length }})</span></h3>
      </header>
      <p v-if="detail.files.length === 0" class="px-4 py-3 text-sm text-muted">{{ t('android.prov.noFiles') }}</p>
      <ul v-else>
        <li v-for="f in detail.files" :key="f.id" class="flex items-center gap-3 px-4 py-2 border-t border-border first:border-t-0 text-xs">
          <StatusBadge :label="fileKindLabel(f.kind)" variant="neutral" />
          <a :href="fileUrl(repo, v.id, f.id)" class="min-w-0 flex-1 truncate font-mono text-primary hover:underline" :download="f.name">{{ f.name }}</a>
          <span class="font-mono text-muted">{{ formatBytes(f.size_bytes) }}</span>
          <button v-if="canWrite" type="button" class="text-danger hover:opacity-80" :aria-label="`${t('common.delete')} ${f.name}`" @click="remove(f.id)">
            <TrashIcon class="w-4 h-4" aria-hidden="true" />
          </button>
        </li>
      </ul>
      <form v-if="canWrite" class="flex flex-wrap items-end gap-2 px-4 py-3 border-t border-border" @submit.prevent="attach">
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.prov.fileKind') }}
          <select v-model="fileKind" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground">
            <option v-for="k in FILE_KINDS" :key="k" :value="k">{{ fileKindLabel(k) }}</option>
          </select>
        </label>
        <input ref="picker" type="file" class="text-xs text-foreground" :aria-label="t('android.prov.chooseFile')" @change="(e) => (chosen = (e.target as HTMLInputElement).files?.[0] ?? null)" />
        <AppButton type="submit" size="sm" variant="secondary" :disabled="!chosen" :loading="attaching">{{ t('android.prov.attach') }}</AppButton>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { TrashIcon } from '@heroicons/vue/24/outline'
import AppButton from '../../../components/AppButton.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { t } from '../../../i18n/en'
import { useToastStore } from '../../../stores/toast'
import { attachFile, deleteFile, fileUrl } from '../api'
import { formatBytes, originVariant, shortFingerprint } from '../lib'
import { fileKindLabel, originLabel } from '../labels'
import { FILE_KINDS, ORIGINS, type AndroidVersion, type FileKind, type Origin, type VersionDetail, type VersionMetaUpdate } from '../types'

const props = defineProps<{
  repo: string
  detail: VersionDetail
  canWrite: boolean
  busy?: boolean
  /** Other versions of the app: candidates for the parent link. */
  siblings: AndroidVersion[]
}>()
const emit = defineEmits<{ meta: [update: VersionMetaUpdate]; changed: [] }>()

const route = useRoute()
const toasts = useToastStore()
const v = computed(() => props.detail.version)

const origin = ref<Origin>(v.value.origin)
const parentId = ref(v.value.parent_id ?? '')
const sourceUrl = ref(v.value.source_url ?? '')
const channel = ref(v.value.channel)
const pinned = ref(v.value.pinned)
watch(v, (nv) => {
  origin.value = nv.origin
  parentId.value = nv.parent_id ?? ''
  sourceUrl.value = nv.source_url ?? ''
  channel.value = nv.channel
  pinned.value = nv.pinned
})

// An original can only be the parent of another file: not of itself.
const candidates = computed(() => props.siblings.filter((s) => s.id !== v.value.id))

function save(): void {
  emit('meta', {
    origin: origin.value, parent_id: parentId.value, source_url: sourceUrl.value.trim(),
    channel: channel.value.trim(), pinned: pinned.value,
  })
}

const fileKind = ref<FileKind>('other')
const chosen = ref<File | null>(null)
const picker = ref<HTMLInputElement | null>(null)
const attaching = ref(false)

async function attach(): Promise<void> {
  if (!chosen.value) return
  attaching.value = true
  try {
    await attachFile(props.repo, v.value.id, fileKind.value, chosen.value)
    chosen.value = null
    if (picker.value) picker.value.value = ''
    emit('changed')
  } catch (err) {
    toasts.error(t('android.prov.attachFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    attaching.value = false
  }
}

async function remove(id: string): Promise<void> {
  try {
    await deleteFile(props.repo, v.value.id, id)
    emit('changed')
  } catch (err) {
    toasts.error(t('android.prov.deleteFailed'), err instanceof Error ? err.message : undefined)
  }
}
</script>
