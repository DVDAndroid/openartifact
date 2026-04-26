// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { ref } from 'vue'
import { search } from '../api/search'
import type { SearchPackageHit, SearchRepoHit } from '../api/search'

export function useSearch() {
  const query = ref('')
  const repoResults = ref<SearchRepoHit[]>([])
  const packageResults = ref<SearchPackageHit[]>([])
  const loading = ref(false)
  const searched = ref(false)

  let debounceTimer: ReturnType<typeof setTimeout> | undefined
  let requestId = 0

  function scheduleSearch(delayMs = 300): void {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => execute(), delayMs)
  }

  async function execute(): Promise<void> {
    const q = query.value.trim()
    if (!q) {
      clear()
      return
    }
    const currentRequestId = ++requestId
    loading.value = true
    searched.value = true
    try {
      const res = await search({ q, limit: 20 })
      if (currentRequestId !== requestId) return
      repoResults.value = res.repositories || []
      packageResults.value = res.packages || []
    } finally {
      if (currentRequestId === requestId) loading.value = false
    }
  }

  function clear(): void {
    query.value = ''
    repoResults.value = []
    packageResults.value = []
    searched.value = false
    clearTimeout(debounceTimer)
    debounceTimer = undefined
  }

  return { query, repoResults, packageResults, loading, searched, scheduleSearch, execute, clear }
}
