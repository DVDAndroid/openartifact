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
          <span class="text-foreground/70">{{ t('nav.groups') }}</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">{{ t('admin.groups.title') }}</h1>
        <p class="mt-1 text-sm text-muted">Organize users into groups for permission management</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="openCreate"
          class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
        >
          {{ t('admin.groups.create') }}
        </button>
      </div>
    </div>

    <!-- StatRow -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-primary bg-primary-soft">
          <FolderIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Total Groups</div>
          <div class="text-xl font-bold tabular-nums">{{ total }}</div>
        </div>
      </div>
      <div class="bg-surface rounded-xl border border-border p-4 flex items-center gap-3">
        <span class="w-9 h-9 rounded-lg flex items-center justify-center text-success bg-success-soft/60">
          <UsersIcon class="w-4 h-4" />
        </span>
        <div>
          <div class="text-[10px] uppercase tracking-[0.14em] font-semibold text-muted">Total Members</div>
          <div class="text-xl font-bold tabular-nums">{{ totalMembers }}</div>
        </div>
      </div>
    </div>

    <!-- TableCard -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 class="text-sm font-semibold tracking-tight">Groups</h3>
        <span class="text-[11px] font-mono text-muted">{{ groups.length }} total</span>
      </div>
      <div v-if="loading" class="px-4 py-8 text-center text-muted">Loading...</div>
      <div v-else-if="groups.length === 0" class="px-4 py-8 text-center text-muted">No groups found</div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-[11px] uppercase tracking-[0.12em] font-semibold text-muted bg-surface-hover/30">
              <th class="text-left px-4 py-2.5 font-medium">Name</th>
              <th class="text-left px-4 py-2.5 font-medium">Members</th>
              <th class="text-left px-4 py-2.5 font-medium">Created</th>
              <th class="text-left px-4 py-2.5 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="group in groups" :key="group.id" class="hover:bg-surface-hover/40 transition-colors">
              <td class="px-4 py-3">
                <span class="font-semibold text-foreground">{{ group.name }}</span>
              </td>
              <td class="px-4 py-3">
                <button
                  @click="openMembers(group)"
                  class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-mono bg-surface-hover text-foreground hover:bg-surface-hover/80 transition-colors"
                >
                  {{ group.member_count || 0 }}
                </button>
              </td>
              <td class="px-4 py-3 text-xs text-muted font-mono">{{ formatDate(group.created_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1">
                  <button
                    @click="openMembers(group)"
                    title="Manage members"
                    class="p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
                  >
                    <UsersIcon class="w-4 h-4" />
                  </button>
                  <button
                    @click="deleteOne(String(group.id))"
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

    <!-- Create Drawer -->
    <Teleport to="body">
      <div v-if="createDrawerOpen" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="closeCreateDrawer" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">Group</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">Create Group</h2>
            </div>
            <button @click="closeCreateDrawer" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Name</label>
              <input
                v-model="newGroupName"
                type="text"
                placeholder="Group name"
                class="w-full px-3 py-2 border border-border rounded-lg bg-surface-hover/40 text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button
              @click="closeCreateDrawer"
              class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
            >
              Cancel
            </button>
            <button
              @click="submitCreate"
              class="px-3 py-1.5 text-sm rounded-lg bg-primary text-primary-contrast hover:opacity-90 transition-opacity font-medium"
            >
              Create
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Members Drawer -->
    <Teleport to="body">
      <div v-if="membersDrawerOpen && selectedGroup" class="fixed inset-0 z-50 flex items-stretch justify-end animate-fade-in">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="closeMembersDrawer" />
        <div class="relative w-full max-w-xl bg-surface border-l border-border shadow-2xl shadow-black/30 flex flex-col animate-slideRight">
          <div class="px-6 py-4 border-b border-border flex items-center justify-between">
            <div>
              <div class="text-[11px] uppercase tracking-[0.14em] text-muted font-mono">Members</div>
              <h2 class="mt-0.5 text-lg font-semibold tracking-tight">{{ selectedGroup.name }}</h2>
            </div>
            <button @click="closeMembersDrawer" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
              <XMarkIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="flex-1 overflow-y-auto nice-scroll px-6 py-5 space-y-5">
            <!-- Search to add members -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Add member</label>
              <div class="relative">
                <MagnifyingGlassIcon class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
                <input
                  v-model="memberSearch"
                  type="search"
                  placeholder="Search users..."
                  class="w-full pl-9 pr-3 h-9 text-sm rounded-lg bg-surface-hover/40 border border-border text-foreground placeholder-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
                />
              </div>
              <div v-if="memberSearch" class="mt-2 space-y-1 max-h-40 overflow-y-auto">
                <button
                  v-for="user in filteredAvailableUsers"
                  :key="user.id"
                  @click="addMember(user)"
                  class="w-full text-left px-3 py-2 rounded-lg text-sm hover:bg-surface-hover transition-colors text-foreground"
                >
                  {{ user.username }}
                </button>
              </div>
            </div>

            <!-- Current members -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">Current members</label>
              <div v-if="currentMembers.length === 0" class="text-sm text-muted">No members</div>
              <div v-else class="space-y-1">
                <div
                  v-for="memberId in currentMembers"
                  :key="memberId"
                  class="flex items-center justify-between px-3 py-2 rounded-lg bg-surface-hover/40 border border-border"
                >
                  <span class="text-sm text-foreground">{{ getMemberName(memberId) }}</span>
                  <button
                    @click="removeMember(memberId)"
                    class="p-1 rounded text-muted hover:text-danger hover:bg-danger/10 transition-colors"
                  >
                    <XMarkIcon class="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          </div>
          <div class="px-6 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
            <button
              @click="closeMembersDrawer"
              class="px-3 py-1.5 text-sm rounded-lg border border-border text-foreground hover:bg-surface-hover transition-colors"
            >
              Close
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
  listGroups,
  createGroup,
  deleteGroup,
  updateGroupMembers,
  listUsers,
  type GroupItem as AdminGroupItem,
  type UserItem,
} from '../../api/admin'
// The runtime API response includes member_count and members
type GroupItem = AdminGroupItem & { member_count?: number; members?: string[] }
import { useToastStore } from '../../stores/toast'
import { t } from '../../i18n/en'
import {
  ChevronRightIcon,
  FolderIcon,
  UsersIcon,
  TrashIcon,
  XMarkIcon,
  MagnifyingGlassIcon,
} from '@heroicons/vue/24/outline'

const toasts = useToastStore()

const groups = ref<GroupItem[]>([])
const allUsers = ref<UserItem[]>([])
const loading = ref(false)
const createDrawerOpen = ref(false)
const membersDrawerOpen = ref(false)
const selectedGroup = ref<GroupItem | null>(null)
const newGroupName = ref('')
const memberSearch = ref('')
const currentMembers = ref<string[]>([])

const total = computed(() => groups.value.length)
const totalMembers = computed(() => groups.value.reduce((sum, g) => sum + (g.member_count || 0), 0))

const filteredAvailableUsers = computed(() => {
  if (!memberSearch.value) return []
  const query = memberSearch.value.toLowerCase()
  return allUsers.value.filter(
    (u) =>
      !currentMembers.value.includes(String(u.id)) &&
      (u.username?.toLowerCase().includes(query) ||       u.full_name?.toLowerCase().includes(query))
  )
})

function formatDate(date?: string): string {
  if (!date) return '—'
  return new Date(date).toLocaleDateString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

function getMemberName(memberId: string): string {
  const user = allUsers.value.find((u) => u.id === memberId)
  return user?.full_name || user?.username || memberId
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [groupsRes, usersRes] = await Promise.all([
      listGroups({ limit: 1000 }),
      listUsers({ limit: 1000 }),
    ])
    groups.value = groupsRes.items
    allUsers.value = usersRes.items
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load data')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  newGroupName.value = ''
  createDrawerOpen.value = true
}

function closeCreateDrawer(): void {
  createDrawerOpen.value = false
  newGroupName.value = ''
}

async function submitCreate(): Promise<void> {
  if (!newGroupName.value.trim()) {
    toasts.error('Group name is required')
    return
  }
  try {
    await createGroup({ name: newGroupName.value })
    toasts.success('Group created')
    closeCreateDrawer()
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to create group')
  }
}

function openMembers(group: GroupItem): void {
  selectedGroup.value = group
  currentMembers.value = group.members || []
  memberSearch.value = ''
  membersDrawerOpen.value = true
}

function closeMembersDrawer(): void {
  membersDrawerOpen.value = false
  selectedGroup.value = null
  currentMembers.value = []
  memberSearch.value = ''
}

async function addMember(user: UserItem): Promise<void> {
  if (!selectedGroup.value) return
  currentMembers.value.push(String(user.id))
  memberSearch.value = ''
  await updateGroupMembers(String(selectedGroup.value.id), currentMembers.value)
  toasts.success('Member added')
}

async function removeMember(memberId: string): Promise<void> {
  if (!selectedGroup.value) return
  currentMembers.value = currentMembers.value.filter((id) => id !== memberId)
  await updateGroupMembers(String(selectedGroup.value.id), currentMembers.value)
  toasts.success('Member removed')
}

async function deleteOne(id: string): Promise<void> {
  if (!window.confirm('Delete this group?')) return
  try {
    await deleteGroup(id)
    toasts.success('Group deleted')
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Delete failed')
  }
}

onMounted(load)
</script>
