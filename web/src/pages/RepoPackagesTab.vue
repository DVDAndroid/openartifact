<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-6">
    <PackageUploadZone v-if="session.isAdmin" :repo="name" :format="format" @uploaded="load" />

    <div v-if="loading" class="text-muted">{{ t('common.loading') }}</div>
    <template v-else-if="store.packages.length === 0">
      <EmptyState :title="t('packages.empty')" :description="t('packages.emptyHint')" />
    </template>
    <template v-else>
      <DataTable :columns="columns" :rows="store.packages as Record<string, unknown>[]" row-key="name">
        <template #cell-name="{ row }">
          <router-link
            :to="`/repos/${name}/packages/${row.format}/${row.name}`"
            class="text-primary hover:opacity-80 font-medium"
          >
            {{ row.name }}
          </router-link>
        </template>
      </DataTable>
      <Pagination :total="store.total" :page="page" :limit="limit" @change="onPageChange" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRepositoriesStore } from '../stores/repositories'
import { useSessionStore } from '../stores/session'
import DataTable from '../components/DataTable.vue'
import type { Column } from '../components/DataTable.vue'
import Pagination from '../components/Pagination.vue'
import EmptyState from '../components/EmptyState.vue'
import PackageUploadZone from '../components/PackageUploadZone.vue'
import { t } from '../i18n/en'

const props = defineProps<{ name: string; format?: string }>()

const store = useRepositoriesStore()
const session = useSessionStore()
const loading = ref(false)
const page = ref(1)
const limit = ref(25)
let loadGeneration = 0

const columns: Column[] = [
  { key: 'name', label: t('packages.column.name'), sortable: true },
  { key: 'version', label: t('packages.column.version') },
  { key: 'format', label: t('packages.column.format') },
  { key: 'created_at', label: t('packages.column.uploaded') },
]

async function load(): Promise<void> {
  const generation = ++loadGeneration
  loading.value = true
  try {
    await store.fetchPackages(props.name, { page: page.value, limit: limit.value })
  } finally {
    if (generation === loadGeneration) loading.value = false
  }
}

async function onPageChange(newPage: number): Promise<void> {
  page.value = newPage
  await load()
}

onMounted(load)
</script>
