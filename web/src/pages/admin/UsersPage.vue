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
          <span class="text-foreground/70">{{ t('nav.users') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.users.title') }}</h1>
        <p class="mt-1 text-sm text-muted">Manage user accounts and access control</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="openCreate"
          class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
        >
          {{ t('admin.users.create') }}
        </button>
      </div>
    </div>

    <!-- StatRow -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft">
          <UsersIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Total Users</div>
          <div class="text-xl font-bold tabular-nums">{{ total }}</div>
        </div>
      </div>
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-success bg-success-soft/60">
          <CheckIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Active</div>
          <div class="text-xl font-bold tabular-nums">{{ activeCount }}</div>
        </div>
      </div>
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-danger bg-danger-soft/60">
          <NoSymbolIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Disabled</div>
          <div class="text-xl font-bold tabular-nums">{{ disabledCount }}</div>
        </div>
      </div>
    </div>

    <!-- Search -->
    <div class="flex items-center gap-2">
      <div class="relative flex-1 max-w-sm">
        <MagnifyingGlassIcon class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
        <input
          v-model="q"
          type="search"
          placeholder="Search users..."
          class="w-full pl-9 pr-3 h-9 text-sm rounded-lg bg-surface-hover/40 border border-border text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
          @input="onSearch"
        />
      </div>
    </div>

    <!-- TableCard -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Users</h3>
        <span class="text-[11px] font-mono text-muted">{{ filteredUsers.length }} total</span>
      </div>
      <div v-if="loading" class="px-4 py-8 text-center text-muted">Loading...</div>
      <div v-else-if="filteredUsers.length === 0" class="px-4 py-8 text-center text-muted">No users found</div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-[11px] uppercase tracking-[0.12em] font-semibold text-muted bg-surface-hover/30">
              <th class="text-left px-4 py-2.5 font-medium">Username</th>
              <th class="text-left px-4 py-2.5 font-medium">Email</th>
              <th class="text-left px-4 py-2.5 font-medium">Status</th>
              <th class="text-left px-4 py-2.5 font-medium">Created</th>
              <th class="text-left px-4 py-2.5 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="user in filteredUsers" :key="user.id" class="hover:bg-surface-hover/40 transition-colors">
              <td class="px-4 py-3">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-full bg-primary/20 text-primary text-xs font-bold flex items-center justify-center">
                    {{ (user.username || '?')[0].toUpperCase() }}
                  </div>
                  <span class="font-mono text-[13px] font-medium">{{ user.username }}</span>
                </div>
              </td>
              <td class="px-4 py-3 text-muted text-sm">{{ user.email || '—' }}</td>
              <td class="px-4 py-3">
                <span
                  v-if="user.disabled"
                  class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium bg-danger-soft/40 text-danger"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-danger" />
                  Disabled
                </span>
                <span v-else class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium bg-success-soft/40 text-success">
                  <span class="w-1.5 h-1.5 rounded-full bg-success" />
                  Active
                </span>
              </td>
              <td class="px-4 py-3 text-xs text-muted font-mono">{{ formatDate(user.created_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1">
                  <button
                    @click="openEdit(user)"
                    title="Edit"
                    class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  >
                    <PencilIcon class="w-4 h-4" />
                  </button>
                  <button
                    @click="toggleDisable(user)"
                    :title="user.disabled ? 'Enable' : 'Disable'"
                    class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  >
                    <component :is="user.disabled ? CheckIcon : NoSymbolIcon" class="w-4 h-4" />
                  </button>
                  <button
                    @click="forceReset(user.id)"
                    title="Force password reset"
                    class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  >
                    <KeyIcon class="w-4 h-4" />
                  </button>
                  <button
                    @click="deleteOne(user.id)"
                    title="Delete"
                    class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                  >
                    <TrashIcon class="w-4 h-4" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create/Edit Drawer -->
    <Teleport to="body">
      <div v-if="drawerOpen" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="closeDrawer" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">User</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">{{ editingUser ? 'Edit User' : 'Add User' }}</h2>
            </div>
            <button @click="closeDrawer" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Name</label>
              <input
                v-model="formData.full_name"
                type="text"
                placeholder="Display name"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Username</label>
              <input
                v-model="formData.username"
                type="text"
                placeholder="username"
                :disabled="!!editingUser"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50 disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Email</label>
              <input
                v-model="formData.email"
                type="email"
                placeholder="user@example.com"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
            <div v-if="!editingUser">
              <label class="block text-sm font-medium text-foreground mb-1.5">Password</label>
              <input
                v-model="formData.password"
                type="password"
                placeholder="Minimum 12 characters"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
            <label class="flex items-center gap-2 text-sm text-foreground cursor-pointer">
              <input v-model="formData.force_change_password" type="checkbox" class="rounded" />
              <span>Force password change on next login</span>
            </label>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button
              @click="closeDrawer"
              class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
            >
              Cancel
            </button>
            <button
              @click="submitForm"
              class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
            >
              {{ editingUser ? 'Update' : 'Create' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, Teleport } from 'vue'
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  setUserDisabled,
  forceUserPasswordReset,
  type UserItem as AdminUserItem,
} from '../../api/admin'
import type { User } from '../../api/generated/types.gen'
type UserItem = User & { id: string }
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import {
  ChevronRightIcon,
  UsersIcon,
  CheckIcon,
  NoSymbolIcon,
  MagnifyingGlassIcon,
  PencilIcon,
  KeyIcon,
  TrashIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'

const toasts = useToastStore()

const users = ref<UserItem[]>([])
const loading = ref(false)
const q = ref('')
const drawerOpen = ref(false)
const editingUser = ref<UserItem | null>(null)

const formData = ref({
  username: '',
  full_name: '',
  email: '',
  password: '',
  force_change_password: false,
})

const filteredUsers = computed(() => {
  if (!q.value) return users.value
  const query = q.value.toLowerCase()
  return users.value.filter(
    (u) =>
      u.username?.toLowerCase().includes(query) ||
      u.email?.toLowerCase().includes(query) ||
      u.full_name?.toLowerCase().includes(query)
  )
})

const total = computed(() => users.value.length)
const activeCount = computed(() => users.value.filter((u) => !u.disabled).length)
const disabledCount = computed(() => users.value.filter((u) => u.disabled).length)

let searchHandle: ReturnType<typeof setTimeout> | null = null
function onSearch(): void {
  if (searchHandle) clearTimeout(searchHandle)
  searchHandle = setTimeout(() => {
    // Filter is done in computed property
  }, 200)
}

function formatDate(date?: string): string {
  if (!date) return '—'
  return new Date(date).toLocaleDateString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listUsers({ limit: 1000 })
    users.value = res.items as UserItem[]
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load users')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editingUser.value = null
  formData.value = {
    username: '',
    full_name: '',
    email: '',
    password: '',
    force_change_password: false,
  }
  drawerOpen.value = true
}

function openEdit(user: UserItem): void {
  editingUser.value = user
  formData.value = {
    username: user.username || '',
    full_name: user.full_name || '',
    email: user.email || '',
    password: '',
    force_change_password: false,
  }
  drawerOpen.value = true
}

function closeDrawer(): void {
  drawerOpen.value = false
  editingUser.value = null
}

async function submitForm(): Promise<void> {
  try {
    if (editingUser.value) {
      await updateUser(editingUser.value.id, {
        full_name: formData.value.full_name || undefined,
        email: formData.value.email || undefined,
      })
      toasts.success('User updated')
    } else {
      if (!formData.value.username || !formData.value.password) {
        toasts.error('Username and password are required')
        return
      }
      await createUser({
        username: formData.value.username,
        full_name: formData.value.full_name || undefined,
        email: formData.value.email || undefined,
        password: formData.value.password,
        force_change_password: formData.value.force_change_password,
      })
      toasts.success('User created')
    }
    closeDrawer()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Operation failed')
  }
}

async function toggleDisable(user: UserItem): Promise<void> {
  try {
    await setUserDisabled(user.id, !user.disabled)
    toasts.success(user.disabled ? 'User enabled' : 'User disabled')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

async function forceReset(id: string | number): Promise<void> {
  try {
    await forceUserPasswordReset(id)
    toasts.success('Password reset required on next login')
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  }
}

async function deleteOne(id: string | number): Promise<void> {
  if (!window.confirm('Delete this user?')) return
  try {
    await deleteUser(id)
    toasts.success('User deleted')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Delete failed')
  }
}

onMounted(load)
</script>
