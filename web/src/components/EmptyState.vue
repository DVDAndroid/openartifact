<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="flex flex-col items-center justify-center py-16 px-4 text-center">
    <!-- Abstract geometric illustration -->
    <svg width="120" height="120" viewBox="0 0 120 120" fill="none" class="mb-6 opacity-40">
      <rect x="30" y="30" width="60" height="60" rx="12" class="stroke-muted" stroke-width="1.5" transform="rotate(12 60 60)" />
      <rect x="35" y="35" width="50" height="50" rx="10" class="stroke-primary" stroke-width="1.5" transform="rotate(-6 60 60)" />
      <circle cx="60" cy="60" r="8" class="fill-primary/20 stroke-primary" stroke-width="1.5" />
    </svg>

    <h3 class="text-lg font-semibold text-foreground mb-1">{{ title }}</h3>
    <p v-if="description" class="text-sm text-muted max-w-sm">{{ description }}</p>
    <router-link
      v-if="actionLabel && actionTo"
      :to="actionTo"
      class="mt-4 inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-contrast text-sm font-medium hover:bg-primary/90 transition-colors active:scale-[0.97]"
    >
      {{ actionLabel }}
    </router-link>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  description?: string
  actionLabel?: string
  actionTo?: string
}>()
</script>
