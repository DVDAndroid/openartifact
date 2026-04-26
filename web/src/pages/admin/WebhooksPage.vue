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
          <span class="text-foreground/70">{{ t('nav.webhooks') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">Webhooks</h1>
        <p class="mt-1 text-sm text-muted">Configure event webhooks for external integrations and CI/CD pipelines.</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="openCreate"
          class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
        >
          <PlusIcon class="w-4 h-4" />
          Add webhook
        </button>
      </div>
    </div>

    <!-- Stat tiles -->
    <div class="grid grid-cols-2 md:grid-cols-3 gap-3">
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft">
          <BellAlertIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Active Hooks</div>
          <div class="text-xl font-bold tabular-nums">{{ activeCount }}</div>
        </div>
      </div>
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-danger bg-danger-soft/60">
          <ExclamationTriangleIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Failing</div>
          <div class="text-xl font-bold tabular-nums">0</div>
        </div>
      </div>
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-success bg-success-soft/60">
          <PaperAirplaneIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Deliveries (24h)</div>
          <div class="text-xl font-bold tabular-nums">0</div>
        </div>
      </div>
    </div>

    <!-- Table card -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Webhooks</h3>
        <span class="text-[11px] font-mono text-muted">{{ webhooks.length }} total</span>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="px-4 py-8 text-center text-muted">Loading...</div>

      <!-- Empty state -->
      <div v-else-if="webhooks.length === 0" class="bg-surface rounded-xl border-0">
        <EmptyState
          title="No webhooks configured"
          description="Create a webhook to receive HTTP notifications when events occur in your repositories."
        />
      </div>

      <!-- Data table -->
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-[11px] uppercase tracking-[0.12em] font-semibold text-muted bg-surface-hover/30">
              <th class="text-left px-4 py-2.5 font-medium">Name</th>
              <th class="text-left px-4 py-2.5 font-medium">URL</th>
              <th class="text-left px-4 py-2.5 font-medium">Events</th>
              <th class="text-left px-4 py-2.5 font-medium">Status</th>
              <th class="text-left px-4 py-2.5 font-medium">Created</th>
              <th class="text-left px-4 py-2.5 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="hook in webhooks" :key="hook.id" class="hover:bg-surface-hover/40 transition-colors">
              <td class="px-4 py-3">
                <span class="font-mono text-[13px] font-medium">{{ hook.name || '—' }}</span>
              </td>
              <td class="px-4 py-3 max-w-[200px]">
                <span class="text-xs text-muted font-mono truncate block" :title="hook.url">{{ hook.url || '—' }}</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span
                    v-for="event in hook.events || []"
                    :key="event"
                    class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium bg-surface-hover text-muted border border-border"
                  >
                    {{ event }}
                  </span>
                  <span v-if="!hook.events?.length" class="text-xs text-muted">—</span>
                </div>
              </td>
              <td class="px-4 py-3">
                <span
                  v-if="hook.active"
                  class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium bg-success-soft/40 text-success"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-success" />
                  Active
                </span>
                <span v-else class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium bg-danger-soft/40 text-danger">
                  <span class="w-1.5 h-1.5 rounded-full bg-danger" />
                  Inactive
                </span>
              </td>
              <td class="px-4 py-3 text-xs text-muted font-mono">{{ formatDate(hook.created_at) }}</td>
              <td class="px-4 py-3">
                <button
                  @click="deleteOne(hook)"
                  title="Delete"
                  class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                >
                  <TrashIcon class="w-4 h-4" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create Drawer -->
    <Teleport to="body">
      <div v-if="drawerOpen" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="closeDrawer" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">Webhook</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">Add Webhook</h2>
            </div>
            <button @click="closeDrawer" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <!-- Name -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Name</label>
              <input
                v-model="form.name"
                type="text"
                placeholder="My webhook"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>

            <!-- URL -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">
                URL <span class="text-danger">*</span>
              </label>
              <input
                v-model="form.url"
                type="url"
                placeholder="https://example.com/webhook"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>

            <!-- Secret -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">
                Secret <span class="text-muted font-normal">(optional)</span>
              </label>
              <input
                v-model="form.secret"
                type="password"
                placeholder="Shared secret for HMAC signing"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>

            <!-- Events checkboxes -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-2">Events</label>
              <div class="space-y-2">
                <label
                  v-for="event in eventOptions"
                  :key="event.value"
                  class="flex items-center gap-2.5 cursor-pointer group"
                >
                  <input
                    :checked="form.events.includes(event.value)"
                    type="checkbox"
                    class="rounded border-border text-primary focus:ring-primary/50"
                    @change="toggleEvent(event.value)"
                  />
                  <div>
                    <span class="text-sm text-foreground group-hover:text-foreground/80 transition-colors">{{ event.label }}</span>
                    <p v-if="event.hint" class="text-[11px] text-muted">{{ event.hint }}</p>
                  </div>
                </label>
              </div>
            </div>

            <!-- SSL Verify toggle -->
            <div class="flex items-center justify-between py-1.5">
              <div>
                <p class="text-sm font-medium text-foreground">SSL Verify</p>
                <p class="text-[11px] text-muted">Verify TLS certificate when delivering payloads</p>
              </div>
              <button
                type="button"
                role="switch"
                :aria-checked="form.ssl_verify"
                class="relative inline-flex h-5 w-9 shrink-0 rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                :class="form.ssl_verify ? 'bg-primary' : 'bg-border'"
                @click="form.ssl_verify = !form.ssl_verify"
              >
                <span
                  class="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform duration-200 mt-0.5"
                  :class="form.ssl_verify ? 'translate-x-4' : 'translate-x-0.5'"
                />
              </button>
            </div>

            <!-- Active toggle -->
            <div class="flex items-center justify-between py-1.5">
              <div>
                <p class="text-sm font-medium text-foreground">Active</p>
                <p class="text-[11px] text-muted">When disabled, no events will be delivered</p>
              </div>
              <button
                type="button"
                role="switch"
                :aria-checked="form.active"
                class="relative inline-flex h-5 w-9 shrink-0 rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                :class="form.active ? 'bg-primary' : 'bg-border'"
                @click="form.active = !form.active"
              >
                <span
                  class="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform duration-200 mt-0.5"
                  :class="form.active ? 'translate-x-4' : 'translate-x-0.5'"
                />
              </button>
            </div>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button
              @click="closeDrawer"
              class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
            >
              Cancel
            </button>
            <button
              @click="submitCreate"
              :disabled="saving || !form.url.trim()"
              class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <ArrowPathIcon v-if="saving" class="w-4 h-4 animate-spin inline mr-1" />
              Save
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, Teleport } from 'vue'
import { request } from '../../api/client'
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import {
  ChevronRightIcon,
  PlusIcon,
  BellAlertIcon,
  ExclamationTriangleIcon,
  PaperAirplaneIcon,
  TrashIcon,
  XMarkIcon,
  ArrowPathIcon,
} from '@heroicons/vue/24/outline'
import EmptyState from '../../components/EmptyState.vue'

interface Webhook {
  id?: string
  repository_id?: string
  name?: string
  url?: string
  secret?: string
  events?: ('push' | 'delete' | 'cleanup')[]
  content_type?: string
  ssl_verify?: boolean
  active?: boolean
  max_retries?: number
  timeout_seconds?: number
  created_at?: string
  updated_at?: string
}

interface WebhookListResponse {
  webhooks?: Webhook[]
}

const toasts = useToastStore()

const webhooks = ref<Webhook[]>([])
const loading = ref(false)
const drawerOpen = ref(false)
const saving = ref(false)

const form = ref({
  name: '',
  url: '',
  secret: '',
  events: [] as ('push' | 'delete' | 'cleanup')[],
  ssl_verify: true,
  active: true,
})

const eventOptions = [
  { value: 'push' as const, label: 'Push', hint: 'Triggered when packages are pushed to a repository' },
  { value: 'delete' as const, label: 'Delete', hint: 'Triggered when packages are deleted from a repository' },
  { value: 'cleanup' as const, label: 'Cleanup', hint: 'Triggered when repository cleanup tasks complete' },
]

const activeCount = computed(() => webhooks.value.filter((h) => h.active).length)

function formatDate(date?: string): string {
  if (!date) return '—'
  return new Date(date).toLocaleDateString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

function toggleEvent(value: 'push' | 'delete' | 'cleanup'): void {
  const idx = form.value.events.indexOf(value)
  if (idx === -1) {
    form.value.events.push(value)
  } else {
    form.value.events.splice(idx, 1)
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await request<WebhookListResponse>('/api/v1/webhooks')
    webhooks.value = res.webhooks ?? []
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load webhooks')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  form.value = {
    name: '',
    url: '',
    secret: '',
    events: [],
    ssl_verify: true,
    active: true,
  }
  drawerOpen.value = true
}

function closeDrawer(): void {
  drawerOpen.value = false
}

async function submitCreate(): Promise<void> {
  if (!form.value.url.trim()) {
    toasts.error('URL is required')
    return
  }
  saving.value = true
  try {
    const payload: Record<string, unknown> = {
      url: form.value.url,
      active: form.value.active,
      ssl_verify: form.value.ssl_verify,
    }
    if (form.value.name.trim()) payload.name = form.value.name.trim()
    if (form.value.secret.trim()) payload.secret = form.value.secret.trim()
    if (form.value.events.length > 0) payload.events = form.value.events

    await request<Webhook>('/api/v1/webhooks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    toasts.success('Webhook created')
    closeDrawer()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to create webhook')
  } finally {
    saving.value = false
  }
}

async function deleteOne(hook: Webhook): Promise<void> {
  if (!window.confirm(`Delete webhook "${hook.name || hook.url}"?`)) return
  try {
    await request(`/api/v1/webhooks/${hook.id}`, { method: 'DELETE' })
    toasts.success('Webhook deleted')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to delete webhook')
  }
}

onMounted(load)
</script>
