// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Repository } from '../api/generated/types.gen'
import type { PackageItem } from '../api/repositories'
import { listRepositories, getRepository, listPackages } from '../api/repositories'

export const useRepositoriesStore = defineStore('repositories', () => {
  const list = ref<Repository[]>([])
  const current = ref<Repository | null>(null)
  const packages = ref<PackageItem[]>([])
  const total = ref(0)
  const loading = ref(false)
  let packageRequestGeneration = 0

  async function fetchList(params?: { page?: number; limit?: number; format?: string }): Promise<void> {
    loading.value = true
    try {
      const result = await listRepositories(params)
      list.value = result.repositories
      total.value = result.total
    } finally {
      loading.value = false
    }
  }

  async function fetchOne(name: string): Promise<void> {
    loading.value = true
    try {
      current.value = await getRepository(name)
    } finally {
      loading.value = false
    }
  }

  async function fetchPackages(name: string, params?: { page?: number; limit?: number; format?: string }): Promise<void> {
    const generation = ++packageRequestGeneration
    loading.value = true
    try {
      const result = await listPackages(name, params)
      if (generation === packageRequestGeneration) {
        packages.value = result.packages
        total.value = result.total
      }
    } finally {
      if (generation === packageRequestGeneration) loading.value = false
    }
  }

  return { list, current, packages, total, loading, fetchList, fetchOne, fetchPackages }
})
