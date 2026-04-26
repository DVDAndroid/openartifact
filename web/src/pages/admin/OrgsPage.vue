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
          <span>Admin</span>
          <ChevronRightIcon class="w-3 h-3" />
          <span class="text-foreground/70">Organizations</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">Organizations</h1>
        <p class="mt-1 text-sm text-muted">Manage organizations and their members.</p>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
        @click="openCreate"
      >
        <PlusIcon class="w-4 h-4" />
        New organization
      </button>
    </div>

    <!-- Stat Tiles -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <div class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0 bg-primary/10">
          <BuildingOfficeIcon class="w-5 h-5 text-primary" />
        </div>
        <div class="min-w-0">
          <p class="text-[10px] uppercase tracking-[0.12em] font-mono font-semibold text-muted truncate">Total Orgs</p>
          <p class="text-xl font-bold tabular-nums text-foreground leading-tight">{{ total }}</p>
        </div>
      </div>
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <div class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0 bg-success/10">
          <UsersIcon class="w-5 h-5 text-success" />
        </div>
        <div class="min-w-0">
          <p class="text-[10px] uppercase tracking-[0.12em] font-mono font-semibold text-muted truncate">Total Members</p>
          <p class="text-xl font-bold tabular-nums text-foreground leading-tight">{{ totalMembers }}</p>
        </div>
      </div>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-5 py-4 border-b border-border">
        <div class="h-4 w-40 rounded skel" />
      </div>
      <div v-for="i in 4" :key="i" class="flex items-center gap-4 px-5 py-4 border-b border-border last:border-0">
        <div class="h-3 w-36 rounded skel" />
        <div class="h-5 w-12 rounded-full skel" />
        <div class="h-3 w-28 rounded skel" />
        <div class="flex-1" />
        <div class="flex gap-1">
          <div class="w-7 h-7 rounded-md skel" />
          <div class="w-7 h-7 rounded-md skel" />
          <div class="w-7 h-7 rounded-md skel" />
        </div>
      </div>
    </div>

    <!-- Table -->
    <div v-else class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-5 py-4 flex items-center gap-2 border-b border-border">
        <h2 class="text-sm font-semibold text-foreground">Organizations</h2>
        <span class="px-1.5 py-0.5 rounded-md bg-surface-hover text-[11px] font-mono font-medium text-muted">
          {{ items.length }}
        </span>
      </div>

      <!-- Empty state -->
      <div v-if="items.length === 0" class="py-16 text-center">
        <div class="w-12 h-12 rounded-full bg-surface-hover mx-auto flex items-center justify-center mb-3">
          <BuildingOfficeIcon class="w-6 h-6 text-muted" />
        </div>
        <p class="text-sm font-medium text-foreground">No organizations yet</p>
        <p class="text-xs text-muted mt-1 mb-4">Create your first organization to group users and repositories.</p>
        <button
          type="button"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity"
          @click="openCreate"
        >
          <PlusIcon class="w-4 h-4" />
          New organization
        </button>
      </div>

      <!-- Data table -->
      <table v-else class="w-full">
        <thead>
          <tr class="bg-surface-hover/30">
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5">
              Name
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Members
            </th>
            <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">
              Created
            </th>
            <th class="text-right text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-5 py-2.5">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-border">
          <tr
            v-for="org in items"
            :key="org.id"
            class="hover:bg-surface-hover/30 transition-colors"
          >
            <!-- Name cell -->
            <td class="px-5 py-3.5">
              <div class="flex items-center gap-2.5">
                <div class="w-8 h-8 rounded-md bg-primary/10 flex items-center justify-center shrink-0">
                  <BuildingOfficeIcon class="w-4 h-4 text-primary" />
                </div>
                <div class="min-w-0">
                  <p class="font-mono text-[13px] font-medium text-foreground truncate">{{ org.name }}</p>
                  <p v-if="org.display_name && org.display_name !== org.name" class="text-[11px] text-muted truncate">
                    {{ org.display_name }}
                  </p>
                </div>
              </div>
            </td>

            <!-- Members cell -->
            <td class="px-3 py-3.5">
              <button
                type="button"
                class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-mono bg-surface-hover text-foreground hover:bg-surface-hover/80 transition-colors"
                @click="openMembers(org)"
              >
                <UsersIcon class="w-3 h-3 text-muted" />
                {{ org.member_count ?? 0 }}
              </button>
            </td>

            <!-- Created cell -->
            <td class="px-3 py-3.5">
              <span class="font-mono text-xs text-muted">{{ formatDate(org.created_at) }}</span>
            </td>

            <!-- Actions cell -->
            <td class="px-5 py-3.5">
              <div class="flex items-center gap-0.5 justify-end">
                <button
                  type="button"
                  title="Manage members"
                  class="p-1.5 rounded-md text-muted hover:text-primary hover:bg-primary/10 transition-colors"
                  @click="openMembers(org)"
                >
                  <UsersIcon class="w-4 h-4" />
                </button>
                <button
                  type="button"
                  title="Edit organization"
                  class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  @click="openEdit(org)"
                >
                  <PencilIcon class="w-4 h-4" />
                </button>
                <button
                  type="button"
                  title="Delete organization"
                  class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                  @click="openDelete(org)"
                >
                  <TrashIcon class="w-4 h-4" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ═══════════════════════════════════════════════════════════════════
         ORG DRAWER (create / edit)
    ════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div v-if="orgDrawerOpen" class="fixed inset-0 z-50 flex justify-end">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm animate-fade-in" @click="closeOrgDrawer" />
        <div class="relative w-full max-w-lg bg-surface border-l border-border shadow-2xl flex flex-col animate-slideRight">
          <!-- Header -->
          <div class="px-6 py-5 border-b border-border shrink-0">
            <p class="text-[10px] uppercase tracking-[0.14em] font-mono text-muted mb-0.5">
              {{ orgDrawerMode === 'create' ? 'New organization' : 'Edit organization' }}
            </p>
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-bold text-foreground tracking-tight">
                {{ orgDrawerMode === 'create' ? 'Create Organization' : `Edit · ${editingOrg?.name}` }}
              </h2>
              <button
                type="button"
                class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors shrink-0"
                @click="closeOrgDrawer"
              >
                <XMarkIcon class="w-5 h-5" />
              </button>
            </div>
          </div>

          <!-- Body -->
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-4">
            <!-- Name (create only) -->
            <div v-if="orgDrawerMode === 'create'">
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Name <span class="text-danger ml-0.5">*</span>
              </label>
              <input
                v-model="orgForm.name"
                type="text"
                placeholder="e.g. my-org"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
              />
              <p class="text-[11px] text-muted mt-1 font-mono">Lowercase letters, numbers, and hyphens only.</p>
            </div>

            <!-- Display name -->
            <div>
              <label class="block text-xs font-semibold text-foreground mb-1.5">Display name</label>
              <input
                v-model="orgForm.display_name"
                type="text"
                placeholder="e.g. My Organization"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
              />
            </div>

            <!-- Description -->
            <div>
              <label class="block text-xs font-semibold text-foreground mb-1.5">Description</label>
              <textarea
                v-model="orgForm.description"
                rows="3"
                placeholder="Optional description"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary resize-none transition-colors"
              />
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-border flex items-center gap-2 shrink-0 bg-surface">
            <div class="flex-1" />
            <button
              type="button"
              class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
              @click="closeOrgDrawer"
            >
              Cancel
            </button>
            <button
              type="button"
              :disabled="orgSaving || (orgDrawerMode === 'create' && !orgForm.name.trim())"
              class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
              @click="saveOrg"
            >
              <ArrowPathIcon v-if="orgSaving" class="w-4 h-4 animate-spin" />
              {{ orgDrawerMode === 'create' ? 'Create organization' : 'Save changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ═══════════════════════════════════════════════════════════════════
         MEMBERS DRAWER
    ════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div v-if="membersDrawerOpen" class="fixed inset-0 z-50 flex justify-end">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm animate-fade-in" @click="closeMembersDrawer" />
        <div class="relative w-full max-w-lg bg-surface border-l border-border shadow-2xl flex flex-col animate-slideRight">
          <!-- Header -->
          <div class="px-6 py-5 border-b border-border shrink-0">
            <p class="text-[10px] uppercase tracking-[0.14em] font-mono text-muted mb-0.5">
              Members
            </p>
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-bold text-foreground tracking-tight">
                {{ managingOrg?.name }}
              </h2>
              <button
                type="button"
                class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors shrink-0"
                @click="closeMembersDrawer"
              >
                <XMarkIcon class="w-5 h-5" />
              </button>
            </div>
          </div>

          <!-- Add member form -->
          <div class="px-6 py-4 border-b border-border bg-surface-hover/20 space-y-3">
            <p class="text-xs font-semibold text-foreground">Add member</p>
            <div class="flex items-center gap-2">
              <input
                v-model="newMemberUserId"
                type="text"
                placeholder="User ID or username"
                class="flex-1 px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                @keydown.enter="addMember"
              />
              <div class="inline-flex p-0.5 rounded-lg bg-surface-hover border border-border shrink-0">
                <button
                  v-for="role in ['member', 'owner']"
                  :key="role"
                  type="button"
                  class="px-2.5 py-1 rounded-md text-xs font-medium transition-all"
                  :class="
                    newMemberRole === role
                      ? 'bg-surface text-foreground shadow-sm'
                      : 'text-muted hover:text-foreground'
                  "
                  @click="newMemberRole = role"
                >
                  {{ role }}
                </button>
              </div>
              <button
                type="button"
                :disabled="!newMemberUserId.trim() || addingMember"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed shrink-0"
                @click="addMember"
              >
                <ArrowPathIcon v-if="addingMember" class="w-4 h-4 animate-spin" />
                <PlusIcon v-else class="w-4 h-4" />
                Add
              </button>
            </div>
          </div>

          <!-- Members list -->
          <div class="flex-1 overflow-y-auto nice-scroll">
            <div v-if="membersLoading" class="px-6 py-8 text-center text-sm text-muted">
              Loading members…
            </div>
            <div v-else-if="members.length === 0" class="px-6 py-12 text-center">
              <UsersIcon class="w-10 h-10 text-muted mx-auto mb-3" />
              <p class="text-sm font-medium text-foreground">No members yet</p>
              <p class="text-xs text-muted mt-1">Add the first member using the form above.</p>
            </div>
            <table v-else class="w-full">
              <thead>
                <tr class="bg-surface-hover/30">
                  <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-6 py-2.5">User</th>
                  <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">Role</th>
                  <th class="text-left text-[11px] uppercase tracking-[0.12em] font-semibold text-muted px-3 py-2.5">Joined</th>
                  <th class="w-10" />
                </tr>
              </thead>
              <tbody class="divide-y divide-border">
                <tr
                  v-for="member in members"
                  :key="member.user_id"
                  class="hover:bg-surface-hover/30 transition-colors group"
                >
                  <td class="px-6 py-3">
                    <div class="flex items-center gap-2">
                      <div class="w-7 h-7 rounded-full bg-primary/20 flex items-center justify-center shrink-0">
                        <span class="text-primary text-xs font-medium">
                          {{ member.username?.[0]?.toUpperCase() ?? '?' }}
                        </span>
                      </div>
                      <span class="font-mono text-[12px] text-foreground">{{ member.username }}</span>
                    </div>
                  </td>
                  <td class="px-3 py-3">
                    <span
                      class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium"
                      :class="member.role === 'owner' ? 'bg-warning-soft text-warning' : 'bg-surface-hover text-muted'"
                    >
                      {{ member.role }}
                    </span>
                  </td>
                  <td class="px-3 py-3">
                    <span class="font-mono text-xs text-muted">{{ formatDate(member.joined_at) }}</span>
                  </td>
                  <td class="px-3 py-3">
                    <button
                      type="button"
                      title="Remove member"
                      class="p-1.5 rounded-md text-muted hover:text-danger hover:bg-danger/10 transition-colors opacity-0 group-hover:opacity-100"
                      @click="removeMember(member.user_id)"
                    >
                      <TrashIcon class="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Footer -->
          <div class="px-6 py-4 border-t border-border shrink-0 bg-surface">
            <button
              type="button"
              class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
              @click="closeMembersDrawer"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ═══════════════════════════════════════════════════════════════════
         DELETE CONFIRMATION MODAL
    ════════════════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="deleteTarget"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in"
      >
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="cancelDelete" />
        <div class="relative bg-surface border border-border rounded-xl shadow-2xl w-full max-w-md">
          <div class="px-6 pt-6 pb-4">
            <h2 class="text-base font-bold text-foreground tracking-tight">Delete organization</h2>
            <p class="text-sm text-muted mt-0.5">This action is permanent and cannot be undone.</p>
          </div>

          <div class="px-6 pb-6 space-y-4">
            <!-- Danger alert -->
            <div class="rounded-lg border border-danger/30 bg-danger-soft p-3">
              <div class="flex items-start gap-2.5">
                <ExclamationCircleIcon class="w-4 h-4 text-danger mt-0.5 shrink-0" />
                <div>
                  <p class="text-xs font-semibold text-danger">All members will be removed</p>
                  <p class="text-[11px] text-danger/80 mt-0.5 leading-relaxed">
                    Deleting this organization will remove all its members and associated data.
                  </p>
                </div>
              </div>
            </div>

            <!-- Confirmation input -->
            <div>
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Type
                <code class="font-mono text-danger bg-danger-soft px-1 py-0.5 rounded mx-1">{{ deleteTarget.name }}</code>
                to confirm
              </label>
              <input
                v-model="deleteConfirmName"
                type="text"
                :placeholder="deleteTarget.name"
                class="w-full px-3 py-2 rounded-lg border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 transition-colors"
                :class="deleteConfirmName && deleteConfirmName !== deleteTarget.name
                  ? 'border-danger focus:ring-danger'
                  : 'border-border focus:ring-danger'"
              />
            </div>

            <div class="flex items-center gap-2 justify-end pt-1">
              <button
                type="button"
                class="px-3 py-1.5 rounded-lg text-sm font-medium text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                @click="cancelDelete"
              >
                Cancel
              </button>
              <button
                type="button"
                :disabled="deleteConfirmName !== deleteTarget.name || deleting"
                class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-danger text-white hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
                @click="confirmDelete"
              >
                <ArrowPathIcon v-if="deleting" class="w-4 h-4 animate-spin" />
                <TrashIcon v-else class="w-4 h-4" />
                Delete organization
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { request } from '../../api/client'
import { useToastStore } from '../../stores/toast'
import {
  BuildingOfficeIcon,
  UsersIcon,
  PlusIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  ArrowPathIcon,
  ChevronRightIcon,
  ExclamationCircleIcon,
} from '@heroicons/vue/24/outline'

// ─── Types ─────────────────────────────────────────────────────────────────────

interface Org {
  id: string
  name: string
  display_name?: string
  description?: string
  member_count?: number
  created_at?: string
}

interface OrgMember {
  user_id: string
  username?: string
  role: string
  joined_at?: string
}

// ─── State ─────────────────────────────────────────────────────────────────────

const toasts = useToastStore()
const items = ref<Org[]>([])
const loading = ref(true)

const total = computed(() => items.value.length)
const totalMembers = computed(() => items.value.reduce((s, o) => s + (o.member_count ?? 0), 0))

// Org drawer
const orgDrawerOpen = ref(false)
const orgDrawerMode = ref<'create' | 'edit'>('create')
const editingOrg = ref<Org | null>(null)
const orgSaving = ref(false)
const orgForm = ref({ name: '', display_name: '', description: '' })

// Members drawer
const membersDrawerOpen = ref(false)
const managingOrg = ref<Org | null>(null)
const members = ref<OrgMember[]>([])
const membersLoading = ref(false)
const newMemberUserId = ref('')
const newMemberRole = ref('member')
const addingMember = ref(false)

// Delete
const deleteTarget = ref<Org | null>(null)
const deleteConfirmName = ref('')
const deleting = ref(false)

// ─── Helpers ───────────────────────────────────────────────────────────────────

function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

// ─── Data Loading ──────────────────────────────────────────────────────────────

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await request<{ items: Org[]; total: number }>('/api/v1/orgs')
    items.value = res?.items ?? []
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load organizations')
  } finally {
    loading.value = false
  }
}

// ─── Org Drawer ────────────────────────────────────────────────────────────────

function openCreate(): void {
  orgForm.value = { name: '', display_name: '', description: '' }
  orgDrawerMode.value = 'create'
  editingOrg.value = null
  orgDrawerOpen.value = true
}

function openEdit(org: Org): void {
  orgForm.value = {
    name: org.name,
    display_name: org.display_name ?? '',
    description: org.description ?? '',
  }
  orgDrawerMode.value = 'edit'
  editingOrg.value = org
  orgDrawerOpen.value = true
}

function closeOrgDrawer(): void {
  orgDrawerOpen.value = false
  editingOrg.value = null
}

async function saveOrg(): Promise<void> {
  if (orgDrawerMode.value === 'create' && !orgForm.value.name.trim()) return
  orgSaving.value = true
  try {
    if (orgDrawerMode.value === 'create') {
      await request('/api/v1/orgs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: orgForm.value.name.trim(),
          display_name: orgForm.value.display_name.trim() || undefined,
          description: orgForm.value.description.trim() || undefined,
        }),
      })
      toasts.success(`Organization "${orgForm.value.name}" created`)
    } else if (editingOrg.value) {
      await request(`/api/v1/orgs/${encodeURIComponent(editingOrg.value.name)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          display_name: orgForm.value.display_name.trim() || undefined,
          description: orgForm.value.description.trim() || undefined,
        }),
      })
      toasts.success(`Organization updated`)
    }
    closeOrgDrawer()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Save failed')
  } finally {
    orgSaving.value = false
  }
}

// ─── Members Drawer ────────────────────────────────────────────────────────────

async function openMembers(org: Org): Promise<void> {
  managingOrg.value = org
  members.value = []
  newMemberUserId.value = ''
  newMemberRole.value = 'member'
  membersDrawerOpen.value = true
  await loadMembers(org.name)
}

function closeMembersDrawer(): void {
  membersDrawerOpen.value = false
  managingOrg.value = null
  members.value = []
}

async function loadMembers(orgName: string): Promise<void> {
  membersLoading.value = true
  try {
    const res = await request<{ items: OrgMember[] }>(
      `/api/v1/orgs/${encodeURIComponent(orgName)}/members`,
    )
    members.value = res?.items ?? []
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load members')
  } finally {
    membersLoading.value = false
  }
}

async function addMember(): Promise<void> {
  const userId = newMemberUserId.value.trim()
  if (!userId || !managingOrg.value) return
  addingMember.value = true
  try {
    await request(`/api/v1/orgs/${encodeURIComponent(managingOrg.value.name)}/members`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId, role: newMemberRole.value }),
    })
    newMemberUserId.value = ''
    toasts.success('Member added')
    await loadMembers(managingOrg.value.name)
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to add member')
  } finally {
    addingMember.value = false
  }
}

async function removeMember(userId: string): Promise<void> {
  if (!managingOrg.value) return
  if (!window.confirm('Remove this member from the organization?')) return
  try {
    await request(
      `/api/v1/orgs/${encodeURIComponent(managingOrg.value.name)}/members/${encodeURIComponent(userId)}`,
      { method: 'DELETE' },
    )
    toasts.success('Member removed')
    await loadMembers(managingOrg.value.name)
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to remove member')
  }
}

// ─── Delete ────────────────────────────────────────────────────────────────────

function openDelete(org: Org): void {
  deleteTarget.value = org
  deleteConfirmName.value = ''
}

function cancelDelete(): void {
  deleteTarget.value = null
  deleteConfirmName.value = ''
}

async function confirmDelete(): Promise<void> {
  if (!deleteTarget.value || deleteConfirmName.value !== deleteTarget.value.name) return
  deleting.value = true
  try {
    await request(`/api/v1/orgs/${encodeURIComponent(deleteTarget.value.name)}`, {
      method: 'DELETE',
    })
    toasts.success(`Organization "${deleteTarget.value.name}" deleted`)
    cancelDelete()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Delete failed')
  } finally {
    deleting.value = false
  }
}

// ─── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(load)
</script>
