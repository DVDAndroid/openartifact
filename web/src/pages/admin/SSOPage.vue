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
          <span class="text-foreground/70">{{ t('nav.sso') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.sso.title') }}</h1>
        <p class="mt-1 text-sm text-muted">{{ t('admin.sso.description') }}</p>
      </div>
      <button
        type="button"
        @click="openAddDrawer"
        class="inline-flex items-center gap-2 px-3 py-1.5 bg-primary text-primary-contrast rounded-lg hover:opacity-90 transition-opacity font-medium text-sm"
      >
        <PlusIcon class="w-4 h-4" />
        {{ t('admin.sso.add') }}
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-muted">{{ t('common.loading') }}</div>

    <!-- Empty State -->
    <div v-else-if="providers.length === 0" class="flex flex-col items-center justify-center py-12">
      <ShieldCheckIcon class="w-12 h-12 text-muted mb-4" />
      <h3 class="text-lg font-semibold text-foreground mb-2">{{ t('admin.sso.empty') }}</h3>
      <p class="text-sm text-muted mb-6">{{ t('admin.sso.emptyHint') }}</p>
      <button
        type="button"
        @click="openAddDrawer"
        class="inline-flex items-center gap-2 px-3 py-1.5 bg-primary text-primary-contrast rounded-lg hover:opacity-90 transition-opacity font-medium text-sm"
      >
        <PlusIcon class="w-4 h-4" />
        {{ t('admin.sso.add') }}
      </button>
    </div>

    <!-- Provider Cards Grid -->
    <div v-else class="grid gap-3 md:grid-cols-2">
        <div
          v-for="provider in providers"
          :key="provider.slug"
          class="bg-surface rounded-xl border border-border p-5 relative overflow-hidden group"
          :class="getAccentBarClass(provider)"
        >
          <!-- Accent Bar -->
          <div
            class="absolute left-0 top-0 bottom-0 w-[3px]"
            :class="getAccentBarColorClass(provider)"
          />

          <!-- Content -->
          <div class="pl-2">
            <!-- Top Row: Display Name + Kind Badge -->
            <div class="flex items-start justify-between mb-3">
              <h3 class="font-semibold text-foreground">{{ provider.display_name }}</h3>
              <span class="font-mono text-[10px] px-1.5 py-0.5 rounded bg-surface-hover text-muted">
                {{ provider.kind.toUpperCase() }}
              </span>
            </div>

            <!-- Slug -->
            <p class="text-[11px] font-mono text-muted mb-3">{{ provider.slug }}</p>

            <!-- Status Row -->
            <div class="flex items-center justify-between mb-4">
              <div class="flex items-center gap-2">
                <span
                  class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium"
                  :class="provider.enabled ? 'bg-success/20 text-success' : 'bg-muted/20 text-muted'"
                >
                  {{ provider.enabled ? 'Enabled' : 'Disabled' }}
                </span>
              </div>
              <p class="text-[11px] text-muted">
                Last tested: {{ provider.updated_at ? formatDate(provider.updated_at) : 'Never' }}
              </p>
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="testProvider(provider.slug)"
                :disabled="testing === provider.slug"
                class="inline-flex items-center gap-1 px-2 py-1.5 text-xs rounded bg-surface-hover text-foreground hover:bg-border transition-colors disabled:opacity-50"
              >
                <BoltIcon class="w-3.5 h-3.5" />
                Test
              </button>
              <button
                type="button"
                @click="openEditDrawer(provider)"
                class="inline-flex items-center gap-1 px-2 py-1.5 text-xs rounded bg-surface-hover text-foreground hover:bg-border transition-colors"
              >
                <PencilIcon class="w-3.5 h-3.5" />
                Edit
              </button>
              <button
                type="button"
                @click="toggleEnabled(provider)"
                class="ml-auto inline-flex items-center gap-1 px-2 py-1.5 text-xs rounded bg-surface-hover text-foreground hover:bg-border transition-colors"
              >
                <span v-if="provider.enabled" class="w-3 h-3 rounded-full bg-success" />
                <span v-else class="w-3 h-3 rounded-full bg-muted" />
              </button>
              <button
                type="button"
                @click="deleteProvider(provider.slug)"
                :disabled="providers.length === 1"
                class="inline-flex items-center gap-1 px-2 py-1.5 text-xs rounded bg-surface-hover text-danger hover:bg-danger/10 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <TrashIcon class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
      </div>
    </div>

    <!-- Add/Edit Drawer -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="drawerOpen"
          class="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
          @click="closeDrawer"
        />
      </Transition>
      <Transition name="slideRight">
        <div
          v-if="drawerOpen"
          class="fixed right-0 top-0 bottom-0 z-50 w-full max-w-xl bg-surface border-l border-border shadow-xl overflow-y-auto"
        >
          <!-- Header -->
          <div class="sticky top-0 bg-surface border-b border-border px-6 py-4 flex items-center justify-between">
            <h2 class="text-lg font-semibold text-foreground">
              {{ editingProvider ? `Edit ${editingProvider.display_name}` : 'Add provider' }}
            </h2>
            <button
              type="button"
              @click="closeDrawer"
              class="text-muted hover:text-foreground transition-colors"
            >
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>

          <!-- Form -->
          <form @submit.prevent="saveProvider" class="p-6 space-y-5">
            <!-- Display Name -->
            <div>
              <label for="display-name" class="block text-sm font-medium text-foreground mb-2">
                Display Name
              </label>
              <input
                id="display-name"
                v-model="form.display_name"
                type="text"
                required
                class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="e.g., Google Workspace"
              />
            </div>

            <!-- Slug -->
            <div>
              <label for="slug" class="block text-sm font-medium text-foreground mb-2">
                Slug
              </label>
              <input
                id="slug"
                v-model="form.slug"
                type="text"
                required
                :disabled="!!editingProvider"
                class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm font-mono text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="e.g., google-workspace"
              />
              <p class="text-xs text-muted mt-1">lowercase, hyphens only</p>
            </div>

            <!-- Kind -->
            <div>
              <label for="kind" class="block text-sm font-medium text-foreground mb-2">
                Kind
              </label>
              <div class="flex gap-2">
                <button
                  type="button"
                  @click="form.kind = 'oidc'"
                  :disabled="!!editingProvider"
                  class="flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors"
                  :class="form.kind === 'oidc'
                    ? 'bg-primary text-primary-contrast'
                    : 'bg-surface-hover text-foreground hover:bg-border disabled:opacity-50 disabled:cursor-not-allowed'"
                >
                  OIDC
                </button>
                <button
                  type="button"
                  @click="form.kind = 'saml'"
                  :disabled="!!editingProvider"
                  class="flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors"
                  :class="form.kind === 'saml'
                    ? 'bg-primary text-primary-contrast'
                    : 'bg-surface-hover text-foreground hover:bg-border disabled:opacity-50 disabled:cursor-not-allowed'"
                >
                  SAML
                </button>
              </div>
            </div>

            <!-- OIDC Fields -->
            <template v-if="form.kind === 'oidc'">
              <div>
                <label for="issuer" class="block text-sm font-medium text-foreground mb-2">
                  Issuer URL
                </label>
                <input
                  id="issuer"
                  v-model="form.oidc_config.issuer"
                  type="url"
                  required
                  class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm font-mono text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="https://accounts.google.com"
                />
              </div>
              <div>
                <label for="client-id" class="block text-sm font-medium text-foreground mb-2">
                  Client ID
                </label>
                <input
                  id="client-id"
                  v-model="form.oidc_config.client_id"
                  type="text"
                  required
                  class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm font-mono text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="your-client-id.apps.googleusercontent.com"
                />
              </div>
              <div>
                <label for="client-secret" class="block text-sm font-medium text-foreground mb-2">
                  Client Secret
                </label>
                <input
                  id="client-secret"
                  v-model="form.oidc_config.client_secret"
                  type="password"
                  required
                  class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm font-mono text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="••••••••••••••••"
                />
              </div>
            </template>

            <!-- SAML Fields -->
            <template v-if="form.kind === 'saml'">
              <div>
                <label for="idp-metadata-url" class="block text-sm font-medium text-foreground mb-2">
                  IDP Metadata URL
                </label>
                <input
                  id="idp-metadata-url"
                  v-model="form.saml_config.idp_metadata_url"
                  type="url"
                  class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm font-mono text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
                  placeholder="https://idp.example.com/metadata.xml"
                />
              </div>
              <div>
                <label for="idp-metadata-xml" class="block text-sm font-medium text-foreground mb-2">
                  IDP Metadata XML
                </label>
                <textarea
                  id="idp-metadata-xml"
                  v-model="form.saml_config.idp_metadata_xml"
                  rows="5"
                  class="w-full px-3 py-2 bg-bg border border-border rounded-lg text-sm font-mono text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent resize-none"
                  placeholder="<?xml version=&quot;1.0&quot;?><EntityDescriptor>...</EntityDescriptor>"
                />
              </div>
            </template>

            <!-- Auto-provision Users -->
            <div class="flex items-center gap-3">
              <button
                type="button"
                @click="form.auto_provision_users = !form.auto_provision_users"
                class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
                :class="form.auto_provision_users ? 'bg-success' : 'bg-muted'"
              >
                <span
                  class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
                  :class="form.auto_provision_users ? 'translate-x-6' : 'translate-x-1'"
                />
              </button>
              <label class="text-sm font-medium text-foreground">
                Auto-provision users
              </label>
            </div>

            <!-- Link by Email -->
            <div class="flex items-center gap-3">
              <button
                type="button"
                @click="form.link_by_email = !form.link_by_email"
                class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
                :class="form.link_by_email ? 'bg-success' : 'bg-muted'"
              >
                <span
                  class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
                  :class="form.link_by_email ? 'translate-x-6' : 'translate-x-1'"
                />
              </button>
              <label class="text-sm font-medium text-foreground">
                Link by email
              </label>
            </div>

            <!-- Footer -->
            <div class="flex gap-3 pt-4 border-t border-border">
              <button
                type="button"
                @click="closeDrawer"
                class="flex-1 px-4 py-2 rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors font-medium text-sm"
              >
                Cancel
              </button>
              <button
                type="submit"
                :disabled="saving"
                class="flex-1 px-4 py-2 rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {{ saving ? 'Saving...' : 'Save' }}
              </button>
            </div>
          </form>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Teleport } from 'vue'
import { ChevronRightIcon } from '@heroicons/vue/20/solid'
import {
  BoltIcon,
  PencilIcon,
  TrashIcon,
  PlusIcon,
  XMarkIcon,
  ShieldCheckIcon,
} from '@heroicons/vue/24/outline'
import { request, ApiError } from '../../api/client'
import { saveSSOProvider, testSSOProvider, type SSOConfigInput } from '../../api/admin'
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'

interface SSOProvider {
  id: string
  slug: string
  kind: 'oidc' | 'saml'
  display_name: string
  enabled: boolean
  oidc_config?: {
    issuer: string
    client_id: string
    client_secret: string
  }
  saml_config?: {
    idp_metadata_url: string
    idp_metadata_xml: string
  }
  claim_mapping?: Record<string, unknown>
  group_mapping?: Record<string, unknown>
  auto_provision_users: boolean
  link_by_email: boolean
  created_at: string
  updated_at: string
}

const toasts = useToastStore()

// State
const providers = ref<SSOProvider[]>([])
const loading = ref(true)
const drawerOpen = ref(false)
const editingProvider = ref<SSOProvider | null>(null)
const saving = ref(false)
const testing = ref<string | null>(null)

const form = ref({
  display_name: '',
  slug: '',
  kind: 'oidc' as 'oidc' | 'saml',
  oidc_config: {
    issuer: '',
    client_id: '',
    client_secret: '',
  },
  saml_config: {
    idp_metadata_url: '',
    idp_metadata_xml: '',
  },
  auto_provision_users: false,
  link_by_email: false,
  enabled: true,
})

// Methods
async function loadProviders(): Promise<void> {
  loading.value = true
  try {
    const response = await request<{ providers: SSOProvider[] }>(
      '/api/v1/admin/sso-providers'
    )
    providers.value = response.providers
  } catch (err) {
    toasts.error(
      err instanceof ApiError ? err.message : 'Failed to load providers'
    )
  } finally {
    loading.value = false
  }
}

function openAddDrawer(): void {
  editingProvider.value = null
  form.value = {
    display_name: '',
    slug: '',
    kind: 'oidc',
    oidc_config: {
      issuer: '',
      client_id: '',
      client_secret: '',
    },
    saml_config: {
      idp_metadata_url: '',
      idp_metadata_xml: '',
    },
    auto_provision_users: false,
    link_by_email: false,
    enabled: true,
  }
  drawerOpen.value = true
}

function openEditDrawer(provider: SSOProvider): void {
  editingProvider.value = provider
  form.value = {
    display_name: provider.display_name,
    slug: provider.slug,
    kind: provider.kind,
    oidc_config: provider.oidc_config || {
      issuer: '',
      client_id: '',
      client_secret: '',
    },
    saml_config: provider.saml_config || {
      idp_metadata_url: '',
      idp_metadata_xml: '',
    },
    auto_provision_users: provider.auto_provision_users,
    link_by_email: provider.link_by_email,
    enabled: provider.enabled,
  }
  drawerOpen.value = true
}

function closeDrawer(): void {
  drawerOpen.value = false
  editingProvider.value = null
}

async function saveProvider(): Promise<void> {
  saving.value = true
  try {
    const config: Record<string, unknown> = {}
    if (form.value.kind === 'oidc') {
      config.oidc_config = form.value.oidc_config
    } else {
      config.saml_config = form.value.saml_config
    }
    config.auto_provision_users = form.value.auto_provision_users
    config.link_by_email = form.value.link_by_email

    if (editingProvider.value) {
      // Update existing provider
      await request<void>(
        `/api/v1/admin/sso-providers/${editingProvider.value.slug}`,
        {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            display_name: form.value.display_name,
            ...config,
          }),
        }
      )
      toasts.success('Provider updated successfully')
    } else {
      // Create new provider
      await saveSSOProvider({
        slug: form.value.slug,
        kind: form.value.kind,
        display_name: form.value.display_name,
        enabled: true,
        config,
      })
      toasts.success('Provider created successfully')
    }

    closeDrawer()
    await loadProviders()
  } catch (err) {
    toasts.error(
      err instanceof ApiError ? err.message : 'Failed to save provider'
    )
  } finally {
    saving.value = false
  }
}

async function testProvider(slug: string): Promise<void> {
  testing.value = slug
  try {
    await testSSOProvider(slug)
    toasts.success('Provider test successful')
  } catch (err) {
    toasts.error(
      err instanceof ApiError ? err.message : 'Provider test failed'
    )
  } finally {
    testing.value = null
  }
}

async function toggleEnabled(provider: SSOProvider): Promise<void> {
  try {
    await request<void>(
      `/api/v1/admin/sso-providers/${provider.slug}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !provider.enabled }),
      }
    )
    provider.enabled = !provider.enabled
    toasts.success(
      provider.enabled ? 'Provider enabled' : 'Provider disabled'
    )
  } catch (err) {
    toasts.error(
      err instanceof ApiError ? err.message : 'Failed to update provider'
    )
  }
}

async function deleteProvider(slug: string): Promise<void> {
  if (!confirm('Are you sure you want to delete this provider?')) {
    return
  }

  try {
    await request<void>(
      `/api/v1/admin/sso-providers/${slug}`,
      { method: 'DELETE' }
    )
    toasts.success('Provider deleted successfully')
    await loadProviders()
  } catch (err) {
    toasts.error(
      err instanceof ApiError ? err.message : 'Failed to delete provider'
    )
  }
}

function getAccentBarClass(provider: SSOProvider): string {
  if (!provider.enabled) return ''
  return ''
}

function getAccentBarColorClass(provider: SSOProvider): string {
  if (!provider.enabled) return 'bg-muted'
  // In a real implementation, you'd check if the provider is working correctly
  // For now, we'll just use success color for enabled providers
  return 'bg-success'
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`

  return date.toLocaleDateString()
}

onMounted(loadProviders)
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slideRight-enter-active,
.slideRight-leave-active {
  transition: transform 0.3s ease;
}

.slideRight-enter-from,
.slideRight-leave-to {
  transform: translateX(100%);
}
</style>
