<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- Tag chips; when editable, tags can be added (with suggestions from the repository) and removed. -->
<template>
  <div class="flex flex-wrap items-center gap-1.5">
    <span
      v-for="tag in modelValue"
      :key="tag"
      class="inline-flex items-center gap-1 rounded-md bg-surface-hover px-2 py-0.5 text-xs font-mono text-foreground"
    >
      {{ tag }}
      <button
        v-if="editable"
        type="button"
        class="text-muted hover:text-danger"
        :aria-label="`${t('android.tags.remove')} ${tag}`"
        @click="remove(tag)"
      >
        <XMarkIcon class="w-3 h-3" aria-hidden="true" />
      </button>
    </span>
    <span v-if="!editable && modelValue.length === 0" class="text-xs text-muted">{{ t('android.tags.none') }}</span>
    <template v-if="editable">
      <input
        v-model="draft"
        :list="listId"
        type="text"
        maxlength="64"
        class="h-7 w-36 rounded-md border border-border-strong bg-surface px-2 text-xs font-mono text-foreground placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary"
        :placeholder="t('android.tags.add')"
        :aria-label="t('android.tags.add')"
        @keydown.enter.prevent="commit"
        @keydown.,.prevent="commit"
        @blur="commit"
      />
      <datalist :id="listId">
        <option v-for="s in suggestions" :key="s" :value="s" />
      </datalist>
      <span v-if="error" role="alert" class="text-xs text-danger">{{ error }}</span>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { XMarkIcon } from '@heroicons/vue/20/solid'
import { t } from '../../../i18n/en'
import { isValidTag } from '../lib'

const props = withDefaults(
  defineProps<{ modelValue: string[]; editable?: boolean; suggestions?: string[] }>(),
  { editable: false, suggestions: () => [] },
)
const emit = defineEmits<{ 'update:modelValue': [tags: string[]] }>()

const draft = ref('')
const error = ref('')
let counter = 0
const listId = `android-tags-${counter++}-${Math.random().toString(36).slice(2, 7)}`

function commit(): void {
  const tag = draft.value.trim().toLowerCase()
  if (!tag) return
  if (!isValidTag(tag)) {
    error.value = t('android.tags.invalid')
    return
  }
  error.value = ''
  draft.value = ''
  if (!props.modelValue.includes(tag)) emit('update:modelValue', [...props.modelValue, tag])
}

function remove(tag: string): void {
  emit('update:modelValue', props.modelValue.filter((x) => x !== tag))
}
</script>
