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
          <span class="text-foreground/70">{{ t('nav.permissions') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.permissions.title') }}</h1>
        <p class="mt-1 text-sm text-muted">Define access control rules for repositories and resources</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="openGrant"
          class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
        >
          {{ t('admin.permissions.grant') }}
        </button>
      </div>
    </div>

    <!-- TableCard -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Permissions</h3>
        <span class="text-[11px] font-mono text-muted">{{ permissions.length }} total</span>
      </div>
      <div v-if="loading" class="px-4 py-8 text-center text-muted">Loading...</div>
      <div v-else-if="permissions.length === 0" class="px-4 py-8 text-center text-muted">No permissions found</div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-[11px] uppercase tracking-[0.12em] font-semibold text-muted bg-surface-hover/30">
              <th class="text-left px-4 py-2.5 font-medium">Principal</th>
              <th class="text-left px-4 py-2.5 font-medium">Scope</th>
              <th class="text-left px-4 py-2.5 font-medium">Role</th>
              <th class="text-left px-4 py-2.5 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="perm in permissions" :key="perm.id" class="hover:bg-surface-hover/40 transition-colors">
              <td class="px-4 py-3">
                <span class="font-mono text-[13px] font-medium">{{ perm.principal }}</span>
              </td>
              <td class="px-4 py-3">
                <span class="font-mono text-[12px] px-2 py-0.5 rounded bg-surface-hover text-foreground/80">
                  {{ perm.scope }}
                </span>
              </td>
              <td class="px-4 py-3">
                <span :class="getRolePillClasses(perm.role)">
                  {{ perm.role }}
                </span>
              </td>
              <td class="px-4 py-3">
                <button
                  @click="revoke(String(perm.id))"
                  title="Revoke"
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

    <!-- Grant Drawer -->
    <Teleport to="body">
      <div v-if="grantDrawerOpen" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="closeGrant" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">Permission</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">Grant Permission</h2>
            </div>
            <button @click="closeGrant" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Principal</label>
              <input
                v-model="formData.principal"
                type="text"
                placeholder="user@example.com or group-name"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
              <p class="mt-1 text-xs text-muted">User email or group name</p>
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Scope</label>
              <input
                v-model="formData.scope"
                type="text"
                placeholder="e.g. repos/*, repos/docker-*"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
              <p class="mt-1 text-xs text-muted">Use wildcards for patterns</p>
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Role</label>
              <select
                v-model="formData.role"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
              >
                <option value="read">Read</option>
                <option value="write">Write</option>
                <option value="admin">Admin</option>
              </select>
            </div>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button
              @click="closeGrant"
              class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
            >
              Cancel
            </button>
            <button
              @click="submitGrant"
              class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
            >
              Grant
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, Teleport } from 'vue'
import {
  listPermissions,
  grantPermission,
  revokePermission,
  type PermissionItem as AdminPermissionItem,
} from '../../api/admin'
import type { PermissionTarget } from '../../api/generated/types.gen'
// The generated PermissionTarget doesn't include principal/scope/role
// which are used by this page; augment for backward compatibility
type PermissionItem = PermissionTarget & { principal: string; scope: string; role: string }
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import {
  ChevronRightIcon,
  TrashIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'

const toasts = useToastStore()

const permissions = ref<PermissionItem[]>([])
const loading = ref(false)
const grantDrawerOpen = ref(false)

const formData = ref({
  principal: '',
  scope: '*',
  role: 'read',
})

function getRolePillClasses(role: string): string {
  const baseClasses = 'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium'
  switch (role) {
    case 'admin':
      return `${baseClasses} bg-danger-soft/40 text-danger`
    case 'write':
      return `${baseClasses} bg-warning-soft/40 text-warning`
    case 'read':
      return `${baseClasses} bg-success-soft/40 text-success`
    default:
      return `${baseClasses} bg-surface-hover text-foreground`
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listPermissions({ limit: 1000 })
    permissions.value = res.items as PermissionItem[]
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load permissions')
  } finally {
    loading.value = false
  }
}

function openGrant(): void {
  formData.value = {
    principal: '',
    scope: '*',
    role: 'read',
  }
  grantDrawerOpen.value = true
}

function closeGrant(): void {
  grantDrawerOpen.value = false
}

async function submitGrant(): Promise<void> {
  if (!formData.value.principal.trim() || !formData.value.scope.trim()) {
    toasts.error('Principal and scope are required')
    return
  }
  try {
    await grantPermission({
      principal: formData.value.principal,
      scope: formData.value.scope,
      role: formData.value.role,
    })
    toasts.success('Permission granted')
    closeGrant()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to grant permission')
  }
}

async function revoke(id: string): Promise<void> {
  if (!window.confirm('Revoke this permission?')) return
  try {
    await revokePermission(id)
    toasts.success('Permission revoked')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to revoke permission')
  }
}

onMounted(load)
</script>
