<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- Signing keys the repository knows: public fingerprints and certificates only, never keystores. -->
<template>
  <div class="space-y-4">
    <p class="text-sm text-muted max-w-2xl">{{ t('android.signers.hint') }}</p>

    <section v-if="canWrite" class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <h3 class="text-sm font-semibold text-foreground">{{ t('android.signers.add') }}</h3>
      <form class="grid gap-3 sm:grid-cols-2" @submit.prevent="add">
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground sm:col-span-2">
          {{ t('android.signers.source') }}
          <textarea v-model="source" rows="3" class="rounded-lg border border-border-strong bg-surface p-2 text-xs font-mono text-foreground" :placeholder="t('android.signers.sourceHint')" />
          <small v-if="sourceError" role="alert" class="text-danger font-normal">{{ sourceError }}</small>
          <small v-else-if="parsed" class="text-muted font-normal font-mono break-all">{{ parsed.kind === 'fingerprint' ? colonFingerprint(parsed.value) : t('android.signers.certificate') }}</small>
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.signers.trust') }}
          <select v-model="trust" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground">
            <option v-for="tr in TRUST_LEVELS" :key="tr" :value="tr">{{ trustLabel(tr) }}</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.signers.label') }}
          <input v-model="label" type="text" maxlength="255" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground" />
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.signers.app') }}
          <input v-model="appName" type="text" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm font-mono text-foreground" :placeholder="t('android.signers.appHint')" />
        </label>
        <label class="flex flex-col gap-1 text-xs font-medium text-foreground">
          {{ t('android.signers.notes') }}
          <input v-model="notes" type="text" class="rounded-lg border border-border-strong bg-surface px-2 py-1.5 text-sm text-foreground" />
        </label>
        <div class="sm:col-span-2 flex justify-end">
          <AppButton type="submit" size="sm" :disabled="!parsed" :loading="saving">{{ t('android.signers.register') }}</AppButton>
        </div>
      </form>
    </section>

    <p v-if="error" role="alert" class="text-sm text-danger">{{ error }}</p>
    <div v-else-if="loading" class="text-sm text-muted">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="signers.length === 0" :title="t('android.signers.emptyTitle')" :description="t('android.signers.emptyHint')" />

    <div v-else class="overflow-x-auto rounded-xl border border-border bg-surface">
      <table class="w-full">
        <thead>
          <tr class="border-b border-border bg-surface-hover text-xs font-semibold text-muted">
            <th class="text-left px-4 py-2.5">{{ t('android.signers.fingerprint') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.signers.label') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.signers.trust') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.signers.scope') }}</th>
            <th class="text-left px-4 py-2.5">{{ t('android.signers.notes') }}</th>
            <th class="text-right px-4 py-2.5"><span class="sr-only">{{ t('common.actions') }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in signers" :key="s.id" class="border-t border-border first:border-t-0 align-top">
            <td class="px-4 py-2.5"><SignerBadge :sha256="s.sha256" :trust="s.trust" known /></td>
            <td class="px-4 py-2.5 text-sm text-foreground">
              <template v-if="editing === s.id"><input v-model="draftLabel" type="text" class="w-40 rounded-md border border-border-strong bg-surface px-2 py-1 text-sm" :aria-label="t('android.signers.label')" /></template>
              <template v-else>{{ s.label || '—' }}</template>
            </td>
            <td class="px-4 py-2.5 text-sm">
              <select v-if="editing === s.id" v-model="draftTrust" class="rounded-md border border-border-strong bg-surface px-1.5 py-1 text-sm" :aria-label="t('android.signers.trust')">
                <option v-for="tr in TRUST_LEVELS" :key="tr" :value="tr">{{ trustLabel(tr) }}</option>
              </select>
              <StatusBadge v-else :label="trustLabel(s.trust)" :variant="trustVariant(s.trust)" />
            </td>
            <td class="px-4 py-2.5 text-xs text-muted">{{ s.app_id ? t('android.signers.scopeApp') : t('android.signers.scopeRepo') }}</td>
            <td class="px-4 py-2.5 text-xs text-muted">{{ s.notes || '—' }}</td>
            <td class="px-4 py-2.5 text-right whitespace-nowrap">
              <template v-if="canWrite">
                <template v-if="editing === s.id">
                  <AppButton size="xs" :loading="saving" @click="saveEdit(s)">{{ t('common.save') }}</AppButton>
                  <AppButton size="xs" variant="ghost" class="ml-1" @click="editing = ''">{{ t('common.cancel') }}</AppButton>
                </template>
                <template v-else>
                  <AppButton size="xs" variant="secondary" @click="startEdit(s)">{{ t('common.edit') }}</AppButton>
                  <AppButton size="xs" variant="danger-outline" class="ml-1" @click="pendingDelete = s">{{ t('common.delete') }}</AppButton>
                </template>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ConfirmDialog
      :open="!!pendingDelete"
      :title="t('common.delete')"
      :description="t('android.signers.confirmDelete')"
      :confirm-label="t('common.delete')"
      :busy="saving"
      @confirm="pendingDelete && remove(pendingDelete)"
      @close="pendingDelete = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppButton from '../../../components/AppButton.vue'
import ConfirmDialog from '../../../components/ConfirmDialog.vue'
import EmptyState from '../../../components/EmptyState.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { t } from '../../../i18n/en'
import { useToastStore } from '../../../stores/toast'
import { createSigner, deleteSigner, listSigners, updateSigner } from '../api'
import { colonFingerprint, extractFingerprint, trustVariant } from '../lib'
import { trustLabel } from '../labels'
import { TRUST_LEVELS, type AndroidSigner, type Trust } from '../types'
import SignerBadge from './SignerBadge.vue'

const props = defineProps<{ repo: string; canWrite: boolean }>()

const toasts = useToastStore()
const signers = ref<AndroidSigner[]>([])
const loading = ref(true)
const error = ref('')
const saving = ref(false)

const source = ref('')
const trust = ref<Trust>('mine')
const label = ref('')
const appName = ref('')
const notes = ref('')
const editing = ref('')
const draftLabel = ref('')
const draftTrust = ref<Trust>('mine')
const pendingDelete = ref<AndroidSigner | null>(null)

// A pasted certificate (PEM) or a fingerprint, possibly the whole keytool output.
const parsed = computed<{ kind: 'fingerprint' | 'certificate'; value: string } | null>(() => {
  const text = source.value.trim()
  if (!text) return null
  if (text.includes('-----BEGIN')) {
    if (/PRIVATE KEY/.test(text)) return null
    return { kind: 'certificate', value: text }
  }
  const fp = extractFingerprint(text)
  return fp ? { kind: 'fingerprint', value: fp } : null
})

const sourceError = computed(() => {
  const text = source.value.trim()
  if (!text || parsed.value) return ''
  return /PRIVATE KEY/.test(text) ? t('android.signers.privateKey') : t('android.signers.invalidSource')
})

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    signers.value = (await listSigners(props.repo)) ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('android.signers.loadFailed')
  } finally {
    loading.value = false
  }
}

async function add(): Promise<void> {
  if (!parsed.value) return
  saving.value = true
  try {
    await createSigner(props.repo, {
      ...(parsed.value.kind === 'certificate' ? { certificate: parsed.value.value } : { sha256: parsed.value.value }),
      trust: trust.value,
      label: label.value.trim(),
      notes: notes.value.trim(),
      app: appName.value.trim() || undefined,
    })
    source.value = label.value = appName.value = notes.value = ''
    await load()
  } catch (err) {
    toasts.error(t('android.signers.saveFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    saving.value = false
  }
}

function startEdit(s: AndroidSigner): void {
  editing.value = s.id
  draftLabel.value = s.label
  draftTrust.value = s.trust
}

async function saveEdit(s: AndroidSigner): Promise<void> {
  saving.value = true
  try {
    await updateSigner(props.repo, s.id, { label: draftLabel.value.trim(), trust: draftTrust.value, notes: s.notes })
    editing.value = ''
    await load()
  } catch (err) {
    toasts.error(t('android.signers.saveFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    saving.value = false
  }
}

async function remove(s: AndroidSigner): Promise<void> {
  saving.value = true
  try {
    await deleteSigner(props.repo, s.id)
    pendingDelete.value = null
    await load()
  } catch (err) {
    toasts.error(t('android.signers.saveFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    saving.value = false
  }
}

watch(() => props.repo, () => void load(), { immediate: true })
</script>
