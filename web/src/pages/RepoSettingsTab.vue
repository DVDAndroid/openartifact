<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="max-w-2xl space-y-6">
    <h2 class="text-lg font-semibold text-foreground">{{ t('nav.settings') }}</h2>
    <form @submit.prevent="handleSave" class="space-y-4 bg-surface border border-border rounded-lg p-6">
      <FormField :label="t('repos.column.name')" id="repo-name" :error="errors.name">
        <input
          id="repo-name"
          v-model="form.name"
          type="text"
          class="w-full px-3 py-2 border border-border rounded-md text-sm bg-bg text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
        />
      </FormField>
      <div class="flex items-center gap-3">
        <input id="repo-public" v-model="form.public" type="checkbox" class="rounded" />
        <label for="repo-public" class="text-sm font-medium text-foreground">{{ t('wizard.public') }}</label>
      </div>
      <button
        type="submit"
        :disabled="loading"
        class="py-2 px-4 bg-primary text-primary-contrast text-sm font-medium rounded-md hover:opacity-90 disabled:opacity-50"
      >
        {{ loading ? t('common.saving') : t('common.save') }}
      </button>
    </form>

    <!-- Distributions / components (deb only) -->
    <div v-if="repo.format === 'deb'" class="bg-surface border border-border rounded-lg p-6 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-semibold text-foreground">{{ t('repos.distributions') }}</h3>
        <button
          type="button"
          class="text-xs text-primary hover:opacity-80"
          @click="addDistribution"
        >
          + {{ t('repos.addDistribution') }}
        </button>
      </div>
      <ul v-if="distributions.length" class="divide-y divide-border">
        <li v-for="(dist, i) in distributions" :key="i" class="flex items-center justify-between py-2 text-sm">
          <span class="text-foreground">{{ dist }}</span>
          <button
            type="button"
            class="text-xs text-danger hover:opacity-80"
            @click="removeDistribution(i)"
          >
            {{ t('common.delete') }}
          </button>
        </li>
      </ul>
      <p v-else class="text-sm text-muted">No distributions configured.</p>

      <div class="flex items-center justify-between pt-4 border-t border-border">
        <h3 class="text-sm font-semibold text-foreground">{{ t('repos.components') }}</h3>
        <button
          type="button"
          class="text-xs text-primary hover:opacity-80"
          @click="addComponent"
        >
          + {{ t('repos.addComponent') }}
        </button>
      </div>
      <ul v-if="components.length" class="divide-y divide-border">
        <li v-for="(comp, i) in components" :key="i" class="flex items-center justify-between py-2 text-sm">
          <span class="text-foreground">{{ comp }}</span>
          <button
            type="button"
            class="text-xs text-danger hover:opacity-80"
            @click="removeComponent(i)"
          >
            {{ t('common.delete') }}
          </button>
        </li>
      </ul>
      <p v-else class="text-sm text-muted">No components configured.</p>
    </div>

    <!-- Storage Quota -->
    <div class="bg-surface border border-border rounded-lg p-6 space-y-4">
      <h3 class="text-sm font-semibold text-foreground">Storage Quota</h3>
      <RepoQuotaTab :repo-name="repo.name" />
    </div>

    <!-- GPG signing status -->
    <div class="bg-surface border border-border rounded-lg p-6">
      <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('repos.gpgStatus') }}</h3>
      <div class="space-y-2 text-sm">
        <div class="flex justify-between">
          <span class="text-muted">{{ t('repos.gpgFingerprint') }}</span>
          <span class="font-mono text-xs text-foreground">{{ gpg?.fingerprint ?? '—' }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-muted">{{ t('repos.gpgLastSign') }}</span>
          <span class="text-foreground">{{ gpg?.last_signed_at ?? '—' }}</span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted">{{ t('repos.gpgHealth') }}</span>
          <StatusBadge
            :label="gpg?.healthy === false ? t('status.degraded') : t('status.healthy')"
            :variant="gpg?.healthy === false ? 'danger' : 'success'"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import type { Repository } from '../api/repositories'
import { updateRepository } from '../api/repositories'
import { request, ApiError } from '../api/client'
import FormField from '../components/FormField.vue'
import StatusBadge from '../components/StatusBadge.vue'
import RepoQuotaTab from './RepoQuotaTab.vue'
import { useToastStore } from '../stores/toast'
import { t } from '../i18n/en'

interface GpgStatus {
  fingerprint?: string
  last_signed_at?: string
  healthy?: boolean
}

const props = defineProps<{ repo: Repository }>()

const form = ref({ name: props.repo.name, public: props.repo.public ?? false })
const errors = ref<Record<string, string>>({})
const loading = ref(false)
const distributions = ref<string[]>([])
const components = ref<string[]>([])
const gpg = ref<GpgStatus | null>(null)
const toasts = useToastStore()

watch(
  () => props.repo,
  (r) => {
    form.value = { name: r.name, public: r.public ?? false }
  },
)

onMounted(async () => {
  if (props.repo.format === 'deb') {
    try {
      const res = await request<{ distributions?: string[]; components?: string[] }>(
        `/api/v1/repositories/${encodeURIComponent(props.repo.name)}/deb`,
      )
      distributions.value = res?.distributions ?? []
      components.value = res?.components ?? []
    } catch (err) {
      if (!(err instanceof ApiError && (err.status === 404 || err.status === 501))) {
        toasts.error('Failed to load distributions')
      }
    }
  }
  try {
    gpg.value = await request<GpgStatus>(
      `/api/v1/repositories/${encodeURIComponent(props.repo.name)}/gpg`,
    )
  } catch {
    gpg.value = null
  }
})

async function handleSave(): Promise<void> {
  errors.value = {}
  loading.value = true
  try {
    await updateRepository(props.repo.name, form.value)
    toasts.success(t('common.save'))
  } catch (err: unknown) {
    toasts.error(err instanceof Error ? err.message : 'Save failed')
  } finally {
    loading.value = false
  }
}

function addDistribution(): void {
  const name = window.prompt(t('repos.addDistribution'))
  if (!name) return
  distributions.value = [...distributions.value, name]
  // Persist via API would happen here when wired. The wizard owns the
  // create-time path; this is the live edit path.
  void persistDeb()
}

function removeDistribution(idx: number): void {
  if (!window.confirm(t('repos.confirmRemove'))) return
  distributions.value = distributions.value.filter((_, i) => i !== idx)
  void persistDeb()
}

function addComponent(): void {
  const name = window.prompt(t('repos.addComponent'))
  if (!name) return
  components.value = [...components.value, name]
  void persistDeb()
}

function removeComponent(idx: number): void {
  if (!window.confirm(t('repos.confirmRemove'))) return
  components.value = components.value.filter((_, i) => i !== idx)
  void persistDeb()
}

async function persistDeb(): Promise<void> {
  try {
    await request<void>(`/api/v1/repositories/${encodeURIComponent(props.repo.name)}/deb`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        distributions: distributions.value,
        components: components.value,
      }),
    })
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Failed'
    toasts.error('Failed to save deb settings', msg)
  }
}
</script>
