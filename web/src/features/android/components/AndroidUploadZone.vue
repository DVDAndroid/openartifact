<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!--
  Upload of APKs: every file is inspected first (nothing is stored) and shown
  as a preview card with the signer, SDK levels and warnings; the options for
  the file (origin, original, tags, ...) are filled in before it is stored.
-->
<template>
  <div
    class="rounded-lg border-2 border-dashed p-5 transition-colors"
    :class="dragOver ? 'border-primary bg-primary-soft/30' : 'border-border bg-surface'"
    @dragenter.prevent="dragOver = true"
    @dragover.prevent="dragOver = true"
    @dragleave.prevent="dragOver = false"
    @drop.prevent="onDrop"
  >
    <div class="flex flex-col items-center text-center">
      <ArrowUpTrayIcon class="w-8 h-8 text-muted mb-2" aria-hidden="true" />
      <p class="text-sm text-foreground">{{ t('android.upload.dropzone') }}</p>
      <p class="text-xs text-muted mt-1">{{ t('android.upload.hint') }}</p>
      <AppButton variant="secondary" size="sm" class="mt-3" @click="picker?.click()">{{ t('upload.browse') }}</AppButton>
      <input ref="picker" type="file" multiple accept=".apk,application/vnd.android.package-archive" class="hidden" @change="onPick" />
    </div>

    <ul v-if="items.length" class="mt-4 space-y-3">
      <li v-for="it in items" :key="it.id" class="rounded-lg border border-border bg-bg p-3" :data-state="it.state">
        <div class="flex items-start gap-3">
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-foreground truncate">{{ it.file.name }}</p>
            <p class="text-xs text-muted">{{ formatBytes(it.file.size) }}</p>
          </div>
          <button
            v-if="it.state !== 'uploading'"
            type="button"
            class="text-muted hover:text-foreground"
            :aria-label="t('common.close')"
            @click="remove(it)"
          >
            <XMarkIcon class="w-4 h-4" aria-hidden="true" />
          </button>
          <button v-else type="button" class="text-xs text-danger hover:opacity-80" @click="it.handle?.abort()">
            {{ t('upload.cancel') }}
          </button>
        </div>

        <p v-if="it.state === 'inspecting'" class="mt-2 text-xs text-muted">{{ t('android.upload.inspecting') }}</p>
        <p v-else-if="it.inspectError" role="alert" class="mt-2 text-xs text-danger">{{ it.inspectError }}</p>

        <template v-if="it.inspect">
          <dl class="mt-3 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-xs">
            <dt class="text-muted">{{ t('android.upload.app') }}</dt>
            <dd class="text-foreground">
              <span class="font-medium">{{ it.inspect.info.label || it.inspect.info.manifest.package }}</span>
              <span class="ml-2 font-mono text-muted">{{ it.inspect.info.manifest.package }}</span>
            </dd>
            <dt class="text-muted">{{ t('android.upload.version') }}</dt>
            <dd class="font-mono text-foreground">{{ it.inspect.info.manifest.version_name }} ({{ it.inspect.info.manifest.version_code }})</dd>
            <dt class="text-muted">{{ t('android.upload.sdk') }}</dt>
            <dd class="font-mono text-foreground">
              {{ it.inspect.info.manifest.min_sdk }} → {{ it.inspect.info.manifest.target_sdk }}
              <span v-if="it.inspect.info.abis.length" class="ml-2 text-muted">{{ it.inspect.info.abis.join(', ') }}</span>
            </dd>
            <dt class="text-muted">{{ t('android.upload.signer') }}</dt>
            <dd class="min-w-0">
              <code v-if="signerOf(it)" class="font-mono text-foreground break-all">{{ colonFingerprint(signerOf(it)) }}</code>
              <span v-else class="text-warning">{{ t('android.upload.unsigned') }}</span>
              <span v-for="f in it.inspect.info.signature.flags ?? []" :key="f" class="ml-2 text-warning">{{ tDynamic(`android.flag.${f}`) }}</span>
            </dd>
          </dl>

          <p v-if="it.inspect.duplicate" role="alert" class="mt-2 text-xs text-warning">{{ t('android.upload.duplicate') }}</p>
          <p v-if="previousNote(it)" class="mt-2 text-xs" :class="previousNote(it)?.warn ? 'text-warning' : 'text-muted'">
            {{ previousNote(it)?.text }}
          </p>
          <ul v-if="it.inspect.info.warnings?.length" class="mt-2 list-disc pl-5 text-xs text-warning">
            <li v-for="w in it.inspect.info.warnings" :key="w">{{ w }}</li>
          </ul>

          <div v-if="it.state === 'ready' || it.state === 'failed'" class="mt-3 grid gap-3 sm:grid-cols-2">
            <label class="field">
              <span>{{ t('android.upload.origin') }}</span>
              <select v-model="it.origin" class="input">
                <option v-for="o in ORIGINS" :key="o" :value="o">{{ originLabel(o) }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('android.upload.channel') }}</span>
              <input v-model="it.channel" type="text" class="input" placeholder="stable" />
            </label>
            <label class="field sm:col-span-2">
              <span>{{ t('android.upload.tags') }}</span>
              <input v-model="it.tags" type="text" class="input font-mono" :placeholder="t('android.upload.tagsHint')" />
            </label>

            <template v-if="it.origin === 'patched'">
              <label class="field sm:col-span-2">
                <span>{{ t('android.upload.original') }}</span>
                <input type="file" accept=".apk" class="input" @change="(e) => (it.original = (e.target as HTMLInputElement).files?.[0] ?? null)" />
                <small class="text-muted">{{ t('android.upload.originalHint') }}</small>
              </label>
              <label v-if="!it.original" class="sm:col-span-2 flex items-center gap-2 text-xs text-foreground">
                <input v-model="it.noOriginal" type="checkbox" class="accent-[rgb(var(--color-primary))]" />
                {{ t('android.upload.noOriginal') }}
              </label>
              <p v-if="!it.original && !it.noOriginal" class="sm:col-span-2 text-xs text-warning">{{ t('android.upload.noOriginalWarn') }}</p>
              <label class="field sm:col-span-2">
                <span>{{ t('android.upload.patchInfo') }}</span>
                <textarea v-model="it.patchInfo" rows="3" class="input font-mono" placeholder='{"tool": "ReVanced CLI", "patches": []}' />
                <small v-if="patchInfoError(it)" role="alert" class="text-danger">{{ patchInfoError(it) }}</small>
              </label>
            </template>

            <label class="field sm:col-span-2">
              <span>{{ t('android.upload.sourceUrl') }}</span>
              <input v-model="it.sourceUrl" type="url" class="input" placeholder="https://" />
            </label>
            <label class="field">
              <span>{{ t('android.upload.changelog') }}</span>
              <textarea v-model="it.changelog" rows="2" class="input" />
            </label>
            <label class="field">
              <span>{{ t('android.upload.notes') }}</span>
              <textarea v-model="it.notes" rows="2" class="input" />
            </label>

            <label v-if="it.conflictSigner" class="sm:col-span-2 flex items-start gap-2 text-xs text-foreground">
              <input v-model="it.acceptNewSigner" type="checkbox" class="mt-0.5 accent-[rgb(var(--color-primary))]" />
              <span>{{ t('android.upload.acceptSigner') }}</span>
            </label>
          </div>
        </template>

        <div v-if="it.state === 'uploading'" class="mt-3">
          <div class="h-1 rounded bg-surface-hover overflow-hidden">
            <div class="h-full bg-primary" :style="{ width: `${percent(it)}%` }" />
          </div>
          <p class="mt-0.5 text-xs text-muted">{{ t('upload.uploading') }} {{ percent(it) }}%</p>
        </div>
        <p v-if="it.error" role="alert" class="mt-2 text-xs text-danger">{{ it.error }}</p>
        <div v-if="it.state === 'done' && it.result" class="mt-2 text-xs">
          <p class="text-success">{{ t('upload.complete') }}</p>
          <ul v-if="it.result.warnings?.length" class="mt-1 list-disc pl-5 text-warning">
            <li v-for="w in it.result.warnings" :key="w">{{ w }}</li>
          </ul>
        </div>

        <div v-if="it.state === 'ready' || it.state === 'failed'" class="mt-3 flex justify-end">
          <AppButton size="sm" :disabled="!!patchInfoError(it) || !!it.inspect?.duplicate" @click="store(it)">{{ t('android.upload.store') }}</AppButton>
        </div>
      </li>
    </ul>
    <div v-if="readyCount > 1" class="mt-3 flex justify-end">
      <AppButton size="sm" @click="storeAll">{{ t('android.upload.storeAll').replace('{n}', String(readyCount)) }}</AppButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ArrowUpTrayIcon } from '@heroicons/vue/24/outline'
import { XMarkIcon } from '@heroicons/vue/20/solid'
import AppButton from '../../../components/AppButton.vue'
import { t, tDynamic } from '../../../i18n/en'
import { useToastStore } from '../../../stores/toast'
import { inspectApk, UploadError, uploadApk, type UploadHandle } from '../api'
import { colonFingerprint, formatBytes, parseTags } from '../lib'
import { originLabel } from '../labels'
import { ORIGINS, type InspectResult, type Origin, type UploadResult } from '../types'

const props = defineProps<{ repo: string }>()
const emit = defineEmits<{ uploaded: [result: UploadResult] }>()

type State = 'inspecting' | 'ready' | 'uploading' | 'done' | 'failed'

interface Item {
  id: number
  file: File
  state: State
  inspect?: InspectResult
  inspectError?: string
  origin: Origin
  original: File | null
  tags: string
  channel: string
  changelog: string
  notes: string
  sourceUrl: string
  patchInfo: string
  noOriginal: boolean
  acceptNewSigner: boolean
  conflictSigner: string
  loaded: number
  total: number
  error?: string
  result?: UploadResult
  handle?: UploadHandle
}

const toasts = useToastStore()
const picker = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)
const items = ref<Item[]>([])
let nextId = 1
// New files start from the options of the last one: a batch is usually one kind.
const last = { origin: 'unknown' as Origin, channel: '', tags: '' }

const readyCount = computed(() => items.value.filter((i) => i.state === 'ready').length)

function signerOf(it: Item): string {
  return it.inspect?.info.signature.signer?.sha256 ?? ''
}

function previousNote(it: Item): { text: string; warn: boolean } | null {
  const prev = it.inspect?.previous
  if (!prev) return null
  const signer = signerOf(it)
  const same = !!signer && prev.signer_sha256 === signer
  const base = t('android.upload.previous')
    .replace('{version}', `${prev.version_name} (${prev.version_code})`)
  return { text: `${base} ${same ? t('android.upload.sameSigner') : t('android.upload.otherSigner')}`, warn: !same }
}

function percent(it: Item): number {
  return it.total ? Math.min(100, Math.round((it.loaded / it.total) * 100)) : 0
}

function patchInfoError(it: Item): string {
  const raw = it.patchInfo.trim()
  if (it.origin !== 'patched' || !raw) return ''
  try {
    const v: unknown = JSON.parse(raw)
    return v && typeof v === 'object' && !Array.isArray(v) ? '' : t('android.upload.patchInfoInvalid')
  } catch {
    return t('android.upload.patchInfoInvalid')
  }
}

async function add(file: File): Promise<void> {
  const it = reactive<Item>({
    id: nextId++, file, state: 'inspecting', origin: last.origin, original: null, tags: last.tags,
    channel: last.channel, changelog: '', notes: '', sourceUrl: '', patchInfo: '', noOriginal: false,
    acceptNewSigner: false, conflictSigner: '', loaded: 0, total: file.size,
  })
  items.value.push(it)
  try {
    it.inspect = await inspectApk(props.repo, file)
    it.state = 'ready'
  } catch (err) {
    it.inspectError = err instanceof Error ? err.message : t('android.upload.inspectFailed')
    it.state = 'failed'
  }
}

function onPick(ev: Event): void {
  const target = ev.target as HTMLInputElement
  for (const f of Array.from(target.files ?? [])) void add(f)
  target.value = ''
}

function onDrop(ev: DragEvent): void {
  dragOver.value = false
  for (const f of Array.from(ev.dataTransfer?.files ?? [])) void add(f)
}

function remove(it: Item): void {
  items.value = items.value.filter((x) => x.id !== it.id)
}

async function store(it: Item): Promise<void> {
  if (!it.inspect || it.state === 'uploading') return
  it.state = 'uploading'
  it.error = undefined
  it.loaded = 0
  last.origin = it.origin
  last.channel = it.channel
  last.tags = it.tags
  it.handle = uploadApk(
    props.repo,
    it.file,
    {
      origin: it.origin,
      original: it.origin === 'patched' ? it.original : null,
      tags: parseTags(it.tags),
      channel: it.channel.trim(),
      changelog: it.changelog,
      notes: it.notes,
      source_url: it.sourceUrl.trim(),
      patch_info: it.origin === 'patched' ? it.patchInfo.trim() : '',
      accept_new_signer: it.acceptNewSigner,
      no_original: it.noOriginal,
    },
    (loaded, total) => {
      it.loaded = loaded
      it.total = total
    },
  )
  try {
    it.result = await it.handle.promise
    it.state = 'done'
    toasts.success(t('upload.complete'), it.file.name)
    emit('uploaded', it.result)
  } catch (err) {
    it.state = 'failed'
    if (err instanceof UploadError && err.status === 409) {
      const signer = err.details.signer_sha256
      it.conflictSigner = typeof signer === 'string' ? signer : ''
      it.error = err.details.duplicate ? t('android.upload.duplicate') : err.message
    } else if (err instanceof Error && (err as UploadError).code === 'aborted') {
      it.error = t('common.cancel')
    } else {
      it.error = err instanceof Error ? err.message : t('upload.failed')
    }
  }
}

async function storeAll(): Promise<void> {
  // One at a time: they share the app rows and the signer policy checks.
  for (const it of items.value.filter((i) => i.state === 'ready')) await store(it)
}
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.75rem; color: rgb(var(--color-foreground)); }
.field > span { font-weight: 500; }
.input {
  width: 100%;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--color-border-strong));
  background: rgb(var(--color-surface));
  padding: 0.4rem 0.6rem;
  font-size: 0.8125rem;
  color: rgb(var(--color-foreground));
}
.input:focus { outline: 2px solid rgb(var(--color-primary)); outline-offset: 0; }
</style>
