<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div>
    <!-- Page Head -->
    <div class="flex items-start justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">{{ t('settings.tokens') }}</h1>
        <p class="text-sm text-muted mt-1">{{ t('settings.tokens.subtitle') }}</p>
      </div>
      <button
        type="button"
        class="px-4 py-2 text-sm font-medium rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity flex items-center gap-2"
        @click="createOpen = true"
      >
        <PlusIcon class="w-4 h-4" />
        {{ t('settings.tokens.create') }}
      </button>
    </div>

    <!-- Token List -->
    <div v-if="!loading" class="mt-5">
      <EmptyState v-if="tokens.length === 0" :title="t('settings.tokens.empty')" />
      <div v-else class="bg-surface rounded-xl border border-border overflow-hidden">
        <!-- List Header -->
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold text-foreground">
            {{ t('settings.tokens.yourTokens') }}
            <span class="text-muted">({{ tokens.length }})</span>
          </h2>
        </div>

        <!-- Table -->
        <table class="w-full">
          <thead>
            <tr class="text-[11px] uppercase tracking-[0.12em] font-semibold text-muted bg-surface-hover/30 border-b border-border">
              <th class="px-6 py-3 text-left">{{ t('admin.tokens.column.label') }}</th>
              <th class="px-6 py-3 text-left">{{ t('admin.tokens.column.created') }}</th>
              <th class="px-6 py-3 text-left">{{ t('admin.tokens.column.lastUsed') }}</th>
              <th class="px-6 py-3 text-left">{{ t('admin.tokens.column.expires') }}</th>
              <th class="px-6 py-3 text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="token in tokens"
              :key="token.id"
              class="border-b border-border hover:bg-surface-hover/40 transition-colors"
            >
              <td class="px-6 py-4">
                <span class="font-mono font-medium">{{ token.label }}</span>
              </td>
              <td class="px-6 py-4 text-sm text-muted">
                {{ token.created_at ? new Date(token.created_at).toLocaleDateString() : '-' }}
              </td>
              <td class="px-6 py-4 text-sm text-muted">
                {{ token.last_used_at ? new Date(token.last_used_at).toLocaleDateString() : t('admin.tokens.never') }}
              </td>
              <td class="px-6 py-4 text-sm" :class="isExpired(token) ? 'text-danger' : 'text-muted'">
                {{ token.expires_at ? new Date(token.expires_at).toLocaleDateString() : t('admin.tokens.never') }}
              </td>
              <td class="px-6 py-4 text-right">
                <button
                  type="button"
                  class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger-soft/40 transition-colors"
                  @click="revoke(token.id)"
                  :title="t('admin.tokens.revoke')"
                >
                  <TrashIcon class="w-4 h-4" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Loading State -->
    <div v-else class="text-muted">{{ t('common.loading') }}</div>

    <!-- Create Token Drawer -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="createOpen"
          class="fixed inset-0 z-50 bg-foreground/30"
          @click.self="createOpen = false"
        />
      </Transition>
      <Transition name="slideRight">
        <div
          v-if="createOpen"
          class="fixed inset-y-0 right-0 z-50 max-w-md bg-surface border-l border-border shadow-2xl flex flex-col"
        >
          <!-- Header -->
          <div class="flex items-center justify-between px-6 py-4 border-b border-border">
            <h2 class="text-lg font-semibold">{{ t('settings.tokens.create') }}</h2>
            <button
              type="button"
              class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
              @click="createOpen = false"
            >
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>

          <!-- Body -->
          <form @submit.prevent="submitCreate" class="flex-1 overflow-y-auto px-6 py-6 space-y-6">
            <!-- Label Field -->
            <div>
              <label for="token-label" class="text-[11px] uppercase tracking-[0.14em] font-semibold text-muted block mb-2">
                {{ t('settings.tokens.label') }}
              </label>
              <input
                id="token-label"
                v-model="newToken.label"
                type="text"
                required
                placeholder="e.g., CLI Token"
                class="w-full h-10 px-3 rounded-lg bg-surface-hover/40 border border-border text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-colors"
              />
            </div>

            <!-- Kind Selector (Access Token only for now) -->
            <div>
              <label class="text-[11px] uppercase tracking-[0.14em] font-semibold text-muted block mb-2">
                {{ t('settings.tokens.kind') }}
              </label>
              <div class="p-3 rounded-lg bg-surface-hover/40 border border-border text-sm text-foreground">
                {{ t('settings.tokens.accessToken') }}
              </div>
            </div>
          </form>

          <!-- Footer -->
          <div class="flex items-center gap-3 px-6 py-4 border-t border-border">
            <button
              type="button"
              class="flex-1 px-4 py-2 text-sm font-medium rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
              @click="createOpen = false"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="submit"
              class="flex-1 px-4 py-2 text-sm font-medium rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
              @click="submitCreate"
            >
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Token Reveal Modal -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="revealed"
          class="fixed inset-0 z-50 bg-foreground/30 flex items-center justify-center p-4"
          @click.self="revealed = null"
        />
      </Transition>
      <Transition name="scale">
        <div
          v-if="revealed"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
        >
          <div class="bg-surface border border-border rounded-lg p-6 w-full max-w-md space-y-4 pointer-events-auto">
            <h2 class="text-lg font-semibold">{{ t('settings.tokens.create') }}</h2>
            <p class="text-sm text-warning">{{ t('admin.tokens.reveal') }}</p>
            <div class="bg-surface-hover rounded-lg p-3 font-mono text-xs break-all text-foreground flex items-start gap-3">
              <span class="flex-1">{{ revealed.token }}</span>
              <button
                type="button"
                class="flex-shrink-0 p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover/60 transition-colors"
                @click="copyToken"
                :title="t('common.copy')"
              >
                <ClipboardDocumentIcon class="w-4 h-4" />
              </button>
            </div>
            <div class="flex justify-end">
              <button
                type="button"
                class="px-4 py-2 text-sm font-medium rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
                @click="revealed = null"
              >
                {{ t('common.close') }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listMyTokens, createMyToken, revokeMyToken, type ApiToken as TokenApi, type CreatedToken as CreatedTokenApi } from '../../api/tokens'
import EmptyState from '../../components/EmptyState.vue'
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import {
  PlusIcon,
  TrashIcon,
  XMarkIcon,
  ClipboardDocumentIcon,
} from '@heroicons/vue/24/outline'
import type { Token, TokenCreateResponse } from '../../api/generated/types.gen'

// The generated Token uses 'name' but the runtime API uses 'label'
type ApiToken = Token & { id: string; label?: string }
type CreatedToken = TokenCreateResponse & { id: string; label?: string }

const tokens = ref<ApiToken[]>([])
const loading = ref(false)
const createOpen = ref(false)
const newToken = ref({ label: '' })
const revealed = ref<CreatedToken | null>(null)
const toasts = useToastStore()

function isExpired(token: ApiToken): boolean {
  return !!(token.expires_at && new Date(token.expires_at) < new Date())
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listMyTokens()
    tokens.value = res.items as ApiToken[]
  } finally {
    loading.value = false
  }
}

async function submitCreate(): Promise<void> {
  try {
    revealed.value = await createMyToken({ name: newToken.value.label }) as CreatedToken
    createOpen.value = false
    newToken.value = { label: '' }
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

async function revoke(id: string | number): Promise<void> {
  if (!window.confirm(t('admin.tokens.revoke') + '?')) return
  try {
    await revokeMyToken(id)
    toasts.success('Revoked')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

async function copyToken(): Promise<void> {
  if (!revealed.value) return
  try {
    await navigator.clipboard.writeText(revealed.value.token ?? '')
    toasts.success('Copied to clipboard')
  } catch (err) {
    toasts.error('Failed to copy')
  }
}

onMounted(load)
</script>

<style scoped>
@keyframes slideRight {
  from {
    transform: translateX(100%);
  }
  to {
    transform: translateX(0);
  }
}

@keyframes slideRightOut {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(100%);
  }
}

@keyframes scale {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

@keyframes scaleOut {
  from {
    opacity: 1;
    transform: scale(1);
  }
  to {
    opacity: 0;
    transform: scale(0.95);
  }
}

.slideRight-enter-active {
  animation: slideRight 0.3s ease-out;
}

.slideRight-leave-active {
  animation: slideRightOut 0.3s ease-in;
}

.scale-enter-active {
  animation: scale 0.2s ease-out;
}

.scale-leave-active {
  animation: scaleOut 0.2s ease-in;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
