<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div>
    <h1 class="text-2xl font-semibold text-foreground">Search</h1>

    <!-- Search Input -->
    <div class="w-full mt-4 flex items-center gap-3 px-5 py-3 rounded-full bg-surface border border-border focus-within:border-primary/50 focus-within:ring-2 focus-within:ring-primary/20 transition-all">
      <MagnifyingGlassIcon class="w-5 h-5 text-muted flex-shrink-0" />
      <input
        v-model="query"
        type="text"
        placeholder="Search packages, repositories..."
        class="flex-1 bg-transparent text-foreground placeholder:text-muted outline-none text-sm"
        @input="onInput"
        @keydown.enter="doSearch"
      />
      <button
        v-if="query"
        @click="clearSearch"
        class="flex-shrink-0 text-muted hover:text-foreground transition-colors"
      >
        <XMarkIcon class="w-5 h-5" />
      </button>
    </div>

    <!-- Results Section -->
    <div v-if="searched" class="mt-6 space-y-6">
      <!-- Repositories Results -->
      <div v-if="repoResults.length" class="bg-surface rounded-xl border border-border overflow-hidden">
        <div class="px-5 py-3 border-b border-border flex items-center justify-between">
          <span class="text-sm font-semibold">Repositories</span>
          <span class="text-xs bg-primary/10 text-primary rounded-full px-2 py-0.5">{{ repoResults.length }}</span>
        </div>
        <div>
          <div
            v-for="(repo, idx) in repoResults"
            :key="repo.name"
            :class="[
              'px-5 py-3 hover:bg-surface-hover transition-colors cursor-pointer flex items-center gap-3',
              idx > 0 ? 'border-t border-border' : ''
            ]"
            @click="router.push(`/repos/${repo.name}`)"
          >
            <span class="font-medium text-foreground">{{ repo.name }}</span>
            <FormatBadge :format="repo.format" />
            <span v-if="repo.description" class="text-sm text-muted truncate ml-auto max-w-xs">{{ repo.description }}</span>
          </div>
        </div>
      </div>

      <!-- Packages Results -->
      <div v-if="packageResults.length" class="bg-surface rounded-xl border border-border overflow-hidden">
        <div class="px-5 py-3 border-b border-border flex items-center justify-between">
          <span class="text-sm font-semibold">Packages</span>
          <span class="text-xs bg-primary/10 text-primary rounded-full px-2 py-0.5">{{ packageResults.length }}</span>
        </div>
        <div>
          <div
            v-for="(pkg, idx) in packageResults"
            :key="`${pkg.repository}:${pkg.format}:${pkg.name}:${pkg.version}`"
            :class="[
              'px-5 py-3 hover:bg-surface-hover transition-colors cursor-pointer flex items-center gap-3',
              idx > 0 ? 'border-t border-border' : ''
            ]"
            @click="router.push(`/repos/${pkg.repository}/packages/${pkg.format}/${pkg.name}`)"
          >
            <span class="font-medium text-foreground">{{ pkg.name }}</span>
            <span class="font-mono text-xs text-muted">{{ pkg.version }}</span>
            <FormatBadge :format="pkg.format" />
            <span class="text-xs text-muted">{{ pkg.repository }}</span>
            <span v-if="pkg.description" class="text-sm text-muted truncate">{{ pkg.description }}</span>
          </div>
        </div>
      </div>

      <!-- No Results -->
      <EmptyState
        v-if="!repoResults.length && !packageResults.length"
        title="No results found"
        body="Try a different search term"
      />
    </div>

    <!-- Initial State -->
    <EmptyState
      v-else
      title="Search your artifacts"
      body="Enter a query to search across all repositories and packages"
    />
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useSearch } from '../composables/useSearch'
import FormatBadge from '../components/FormatBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { MagnifyingGlassIcon, XMarkIcon } from '@heroicons/vue/24/outline'

const router = useRouter()
const { query, repoResults, packageResults, searched, scheduleSearch, execute, clear } = useSearch()

function onInput() {
  scheduleSearch()
}

function doSearch() {
  execute()
}

function clearSearch() {
  clear()
}
</script>
