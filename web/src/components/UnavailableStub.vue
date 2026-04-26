<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="flex items-center justify-center py-16">
    <div class="max-w-md w-full bg-surface rounded-lg border border-border p-8 text-center">
      <p class="text-lg font-semibold text-foreground">{{ t('common.notAvailable') }}</p>
      <p class="mt-2 text-sm text-muted">
        {{ t('common.notAvailableHint') }}
      </p>
      <a
        v-if="docsLink"
        :href="docsLink"
        target="_blank"
        rel="noopener noreferrer"
        class="mt-4 inline-block text-sm text-primary hover:opacity-80"
      >
        {{ t('common.viewDocs') }}
      </a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { t } from '../i18n/en'

defineProps<{
  docsLink?: string
}>()
</script>
