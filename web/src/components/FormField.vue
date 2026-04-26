<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="flex flex-col gap-1">
    <label :for="id" class="text-sm font-medium text-foreground">{{ label }}</label>
    <slot />
    <p v-if="hint && !error" class="text-xs text-muted">{{ hint }}</p>
    <p v-if="error" :id="`${id}-error`" role="alert" class="text-xs text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  id: string
  error?: string
  hint?: string
}>()
</script>
