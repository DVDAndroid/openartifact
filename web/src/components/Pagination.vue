<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="flex items-center justify-between px-4 py-3 border-t border-border bg-surface">
    <span class="text-sm text-muted">
      {{ t('pagination.showing') }} {{ from }}–{{ to }} {{ t('pagination.of') }} {{ total }}
    </span>
    <div class="flex gap-2">
      <button
        type="button"
        class="px-3 py-1 text-sm rounded border border-border bg-surface text-foreground disabled:opacity-40 disabled:cursor-not-allowed hover:enabled:bg-surface-hover focus-visible:ring-2 focus-visible:ring-primary"
        :disabled="page <= 1"
        :aria-label="t('pagination.previous')"
        @click="emit('change', page - 1)"
      >
        {{ t('pagination.previous') }}
      </button>
      <button
        type="button"
        class="px-3 py-1 text-sm rounded border border-border bg-surface text-foreground disabled:opacity-40 disabled:cursor-not-allowed hover:enabled:bg-surface-hover focus-visible:ring-2 focus-visible:ring-primary"
        :disabled="page * limit >= total"
        :aria-label="t('pagination.next')"
        @click="emit('change', page + 1)"
      >
        {{ t('pagination.next') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { t } from '../i18n/en'

const props = defineProps<{
  total: number
  page: number
  limit: number
}>()

const emit = defineEmits<{
  change: [page: number]
}>()

const from = computed(() => Math.min((props.page - 1) * props.limit + 1, props.total))
const to = computed(() => Math.min(props.page * props.limit, props.total))
</script>
